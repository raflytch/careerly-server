package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/pkg/genai"

	"github.com/google/uuid"
)

var (
	ErrInterviewNotFound     = errors.New("interview not found")
	ErrInterviewUnauthorized = errors.New("unauthorized access to interview")
	ErrInterviewCompleted    = errors.New("interview already completed")
	ErrInvalidQuestionID     = errors.New("invalid question id")
)

const generateQuestionsPrompt = `You are an expert technical interviewer. Generate interview questions for a %s position.

Requirements:
- Generate exactly %d questions
- Question type: %s
- Questions should be relevant, professional, and assess real-world skills
- For multiple choice, provide exactly 5 options (A, B, C, D, E)
- Each question should have a clear correct answer

Respond ONLY with valid JSON array in this exact format:
[
  {
    "id": 1,
    "type": "%s",
    "question": "Your question here?",
    "options": [
      {"label": "A", "text": "Option A text"},
      {"label": "B", "text": "Option B text"},
      {"label": "C", "text": "Option C text"},
      {"label": "D", "text": "Option D text"},
      {"label": "E", "text": "Option E text"}
    ],
    "correct_answer": "B"
  }
]

For essay type questions, omit the "options" field and provide a brief expected answer in "correct_answer".

Generate questions now:`

const evaluateAnswersPrompt = `You are an expert technical interviewer evaluating interview answers for a %s position.

Here are the questions and the candidate's answers:
%s

Evaluate each answer and provide:
1. For multiple choice: Check if the answer matches the correct answer (true/false)
2. For essay: Evaluate the quality on a scale of 0-100 and provide brief feedback

Respond ONLY with valid JSON array in this exact format:
[
  {
    "question_id": 1,
    "is_correct": true,
    "score": 100,
    "feedback": "Brief feedback explaining the evaluation"
  }
]

Evaluate now:`

type interviewService struct {
	interviewRepo domain.InterviewRepository
	quotaService  domain.QuotaService
	genaiClient   *genai.Client
}

func NewInterviewService(
	interviewRepo domain.InterviewRepository,
	quotaService domain.QuotaService,
	genaiClient *genai.Client,
) domain.InterviewService {
	return &interviewService{
		interviewRepo: interviewRepo,
		quotaService:  quotaService,
		genaiClient:   genaiClient,
	}
}

func (s *interviewService) Create(ctx context.Context, userID uuid.UUID, req *domain.CreateInterviewRequest) (*domain.InterviewResponse, error) {
	if err := s.quotaService.CheckAndIncrementUsage(ctx, userID, domain.FeatureInterview); err != nil {
		return nil, err
	}

	aiStatus := "success"
	questions, err := s.generateQuestions(ctx, req.JobPosition, req.QuestionType, req.QuestionCount)
	if err != nil {
		if s.genaiClient == nil {
			aiStatus = "skipped_no_ai_client"
		} else {
			aiStatus = "failed"
		}
		questions = s.generateFallbackQuestions(req.QuestionType, req.QuestionCount)
	}

	interview := &domain.Interview{
		ID:          uuid.New(),
		UserID:      userID,
		JobPosition: req.JobPosition,
		Questions:   questions,
		Status:      domain.InterviewStatusInProgress,
		CreatedAt:   time.Now(),
	}

	if err := s.interviewRepo.Create(ctx, interview); err != nil {
		return nil, err
	}

	return &domain.InterviewResponse{
		Interview:          s.toInterviewForUser(interview),
		AIGenerationStatus: aiStatus,
	}, nil
}

func (s *interviewService) GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.InterviewForUser, error) {
	interview, err := s.interviewRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInterviewNotFound
		}
		return nil, err
	}

	if interview.UserID != userID {
		return nil, ErrInterviewUnauthorized
	}

	return s.toInterviewForUser(interview), nil
}

func (s *interviewService) GetByUserID(ctx context.Context, userID uuid.UUID, page, limit int) (*domain.PaginatedInterviews, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	total, err := s.interviewRepo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	interviews, err := s.interviewRepo.FindByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	interviewsForUser := make([]domain.InterviewForUser, len(interviews))
	for i, interview := range interviews {
		interviewsForUser[i] = *s.toInterviewForUser(&interview)
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	return &domain.PaginatedInterviews{
		Interviews: interviewsForUser,
		Pagination: domain.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *interviewService) SubmitAnswers(ctx context.Context, userID uuid.UUID, id uuid.UUID, req *domain.SubmitAnswerRequest) (*domain.InterviewResponse, error) {
	interview, err := s.interviewRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInterviewNotFound
		}
		return nil, err
	}

	if interview.UserID != userID {
		return nil, ErrInterviewUnauthorized
	}

	if interview.Status == domain.InterviewStatusCompleted {
		return nil, ErrInterviewCompleted
	}

	answerMap := make(map[int]string)
	for _, ans := range req.Answers {
		answerMap[ans.QuestionID] = ans.Answer
	}

	for i := range interview.Questions {
		if answer, ok := answerMap[interview.Questions[i].ID]; ok {
			interview.Questions[i].UserAnswer = answer
		}
	}

	aiStatus := "success"
	evaluations, err := s.evaluateAnswers(ctx, interview)
	if err != nil {
		if s.genaiClient == nil {
			aiStatus = "skipped_no_ai_client"
		} else {
			aiStatus = "failed"
		}
		evaluations = s.evaluateFallback(interview)
	}

	var totalScore float64
	var answeredCount int
	for i := range interview.Questions {
		for _, eval := range evaluations {
			if eval.QuestionID == interview.Questions[i].ID {
				interview.Questions[i].IsCorrect = eval.IsCorrect
				interview.Questions[i].Score = eval.Score
				interview.Questions[i].Feedback = eval.Feedback
				if eval.Score != nil {
					totalScore += *eval.Score
					answeredCount++
				}
				break
			}
		}
	}

	if answeredCount > 0 {
		avgScore := totalScore / float64(answeredCount)
		interview.OverallScore = &avgScore
	}

	now := time.Now()
	interview.Status = domain.InterviewStatusCompleted
	interview.CompletedAt = &now

	if err := s.interviewRepo.Update(ctx, interview); err != nil {
		return nil, err
	}

	return &domain.InterviewResponse{
		Interview:          s.toInterviewForUser(interview),
		AIEvaluationStatus: aiStatus,
	}, nil
}

func (s *interviewService) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	interview, err := s.interviewRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInterviewNotFound
		}
		return err
	}

	if interview.UserID != userID {
		return ErrInterviewUnauthorized
	}

	return s.interviewRepo.SoftDelete(ctx, id)
}

func (s *interviewService) generateQuestions(ctx context.Context, jobPosition string, questionType domain.QuestionType, count int) ([]domain.Question, error) {
	if s.genaiClient == nil {
		return nil, errors.New("genai client not available")
	}

	typeStr := string(questionType)
	prompt := fmt.Sprintf(generateQuestionsPrompt, jobPosition, count, typeStr, typeStr)

	result, err := s.genaiClient.GenerateJSON(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var questions []domain.Question
	if err := json.Unmarshal([]byte(result), &questions); err != nil {
		return nil, err
	}

	return questions, nil
}

func (s *interviewService) evaluateAnswers(ctx context.Context, interview *domain.Interview) ([]evaluationResult, error) {
	if s.genaiClient == nil {
		return nil, errors.New("genai client not available")
	}

	questionsWithAnswers := make([]map[string]interface{}, 0)
	for _, q := range interview.Questions {
		if q.UserAnswer == "" {
			continue
		}
		qMap := map[string]interface{}{
			"id":             q.ID,
			"type":           q.Type,
			"question":       q.Question,
			"correct_answer": q.CorrectAnswer,
			"user_answer":    q.UserAnswer,
		}
		if len(q.Options) > 0 {
			qMap["options"] = q.Options
		}
		questionsWithAnswers = append(questionsWithAnswers, qMap)
	}

	questionsJSON, err := json.Marshal(questionsWithAnswers)
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(evaluateAnswersPrompt, interview.JobPosition, string(questionsJSON))

	result, err := s.genaiClient.GenerateJSON(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var evaluations []evaluationResult
	if err := json.Unmarshal([]byte(result), &evaluations); err != nil {
		return nil, err
	}

	return evaluations, nil
}

type evaluationResult struct {
	QuestionID int      `json:"question_id"`
	IsCorrect  *bool    `json:"is_correct"`
	Score      *float64 `json:"score"`
	Feedback   string   `json:"feedback"`
}

func (s *interviewService) generateFallbackQuestions(questionType domain.QuestionType, count int) []domain.Question {
	questions := make([]domain.Question, count)
	for i := 0; i < count; i++ {
		q := domain.Question{
			ID:       i + 1,
			Type:     questionType,
			Question: fmt.Sprintf("Sample question %d - Please configure AI service for real questions", i+1),
		}
		if questionType == domain.QuestionTypeMultipleChoice {
			q.Options = []domain.Option{
				{Label: "A", Text: "Option A"},
				{Label: "B", Text: "Option B"},
				{Label: "C", Text: "Option C"},
				{Label: "D", Text: "Option D"},
				{Label: "E", Text: "Option E"},
			}
			q.CorrectAnswer = "A"
		} else {
			q.CorrectAnswer = "Sample expected answer"
		}
		questions[i] = q
	}
	return questions
}

func (s *interviewService) evaluateFallback(interview *domain.Interview) []evaluationResult {
	results := make([]evaluationResult, 0)
	for _, q := range interview.Questions {
		if q.UserAnswer == "" {
			continue
		}
		score := 50.0
		isCorrect := false
		if q.Type == domain.QuestionTypeMultipleChoice {
			isCorrect = q.UserAnswer == q.CorrectAnswer
			if isCorrect {
				score = 100.0
			} else {
				score = 0.0
			}
		}
		results = append(results, evaluationResult{
			QuestionID: q.ID,
			IsCorrect:  &isCorrect,
			Score:      &score,
			Feedback:   "Evaluated using fallback method. Please configure AI for detailed feedback.",
		})
	}
	return results
}

func (s *interviewService) toInterviewForUser(interview *domain.Interview) *domain.InterviewForUser {
	questionsForUser := make([]domain.QuestionForUser, len(interview.Questions))
	for i, q := range interview.Questions {
		questionsForUser[i] = domain.QuestionForUser{
			ID:         q.ID,
			Type:       q.Type,
			Question:   q.Question,
			Options:    q.Options,
			UserAnswer: q.UserAnswer,
			IsCorrect:  q.IsCorrect,
			Score:      q.Score,
			Feedback:   q.Feedback,
		}
	}

	return &domain.InterviewForUser{
		ID:           interview.ID,
		UserID:       interview.UserID,
		JobPosition:  interview.JobPosition,
		Questions:    questionsForUser,
		Status:       interview.Status,
		OverallScore: interview.OverallScore,
		CreatedAt:    interview.CreatedAt,
		CompletedAt:  interview.CompletedAt,
	}
}
