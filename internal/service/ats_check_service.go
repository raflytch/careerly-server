package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mime/multipart"
	"strings"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/pkg/genai"

	"github.com/google/uuid"
)

var (
	ErrATSCheckNotFound     = errors.New("ats check not found")
	ErrATSCheckUnauthorized = errors.New("unauthorized access to ats check")
	ErrAIClientUnavailable  = errors.New("ai client is not available, cannot analyze pdf")
)

const atsFileAnalysisSystemPrompt = `You are an extremely strict and brutally honest ATS (Applicant Tracking System) resume analyzer. Your job is to evaluate resumes the way real ATS software does — with zero sympathy. Do NOT inflate scores. If the resume is bad, say it clearly. If it's mediocre, don't sugarcoat.

Scoring Rules (BE HARSH):
- Missing contact info (email/phone)? Deduct heavily.
- No quantifiable achievements? Score below 50.
- Generic summary with buzzwords but no substance? Penalize.
- Skills listed without evidence in experience? Penalize.
- Gaps, vague descriptions, no action verbs? Penalize.
- Poor formatting indicators (inconsistent dates, missing fields)? Penalize.
- Only give 80+ if the resume is genuinely excellent with quantified achievements, strong action verbs, relevant keywords, and clean structure.
- A score of 90+ should be extremely rare — only for truly outstanding resumes.
- Average resumes should score 40-60. Bad ones below 40.
- If the PDF is poorly formatted, has weird spacing, uses tables/columns that ATS can't parse, or has images instead of text — penalize heavily.

You MUST respond ONLY with valid JSON (no markdown, no backticks, no explanation) in this exact format:
{
  "overall_score": 45.5,
  "verdict": "One sentence brutal honest verdict about this resume",
  "sections": [
    {
      "name": "Contact Information",
      "score": 8,
      "max_score": 10,
      "feedback": "Specific feedback about what's wrong or right"
    },
    {
      "name": "Professional Summary",
      "score": 3,
      "max_score": 15,
      "feedback": "Harsh but actionable feedback"
    },
    {
      "name": "Work Experience",
      "score": 10,
      "max_score": 30,
      "feedback": "Specific issues and what's missing"
    },
    {
      "name": "Education",
      "score": 7,
      "max_score": 10,
      "feedback": "Feedback"
    },
    {
      "name": "Skills",
      "score": 5,
      "max_score": 15,
      "feedback": "Are skills backed by experience?"
    },
    {
      "name": "Achievements & Impact",
      "score": 2,
      "max_score": 10,
      "feedback": "Are there quantified achievements?"
    },
    {
      "name": "Formatting & ATS Compatibility",
      "score": 5,
      "max_score": 10,
      "feedback": "Structure and parsing readability"
    }
  ],
  "keyword_analysis": {
    "found": ["keyword1", "keyword2"],
    "missing": ["important_keyword1", "important_keyword2"],
    "tip": "Specific tip about keyword optimization"
  },
  "improvements": [
    {
      "priority": "critical",
      "category": "Work Experience",
      "issue": "What exactly is wrong",
      "suggestion": "Specific actionable fix"
    },
    {
      "priority": "high",
      "category": "Summary",
      "issue": "What exactly is wrong",
      "suggestion": "Specific actionable fix"
    }
  ],
  "deal_breakers": ["List of things that would immediately get this resume rejected by a recruiter"]
}

Priority levels: "critical", "high", "medium", "low"
Be ruthless. Be specific. No generic advice. Every feedback must reference actual content from this resume PDF.`

const atsFileAnalysisUserPrompt = `Analyze the uploaded resume PDF file as a strict ATS system. Extract all text content from the PDF and evaluate it thoroughly. Be brutally honest — do NOT inflate scores. Respond with the JSON format specified in your instructions.`

type atsCheckService struct {
	atsCheckRepo domain.ATSCheckRepository
	quotaService domain.QuotaService
	genaiClient  *genai.Client
}

func NewATSCheckService(
	atsCheckRepo domain.ATSCheckRepository,
	quotaService domain.QuotaService,
	genaiClient *genai.Client,
) domain.ATSCheckService {
	return &atsCheckService{
		atsCheckRepo: atsCheckRepo,
		quotaService: quotaService,
		genaiClient:  genaiClient,
	}
}

func (s *atsCheckService) AnalyzeFromFile(ctx context.Context, userID uuid.UUID, file *multipart.FileHeader) (*domain.ATSCheckResponse, error) {
	if s.genaiClient == nil {
		return nil, ErrAIClientUnavailable
	}

	if err := s.quotaService.CheckAndIncrementUsage(ctx, userID, domain.FeatureATSCheck); err != nil {
		return nil, err
	}

	analysis, err := s.analyzeFile(ctx, file)
	aiStatus := "success"
	if err != nil {
		aiStatus = "failed"
		analysis = s.buildFallbackAnalysis()
	}

	score := analysis.OverallScore

	check := &domain.ATSCheck{
		ID:        uuid.New(),
		UserID:    userID,
		Score:     &score,
		Analysis:  analysis,
		CreatedAt: time.Now(),
	}

	if err := s.atsCheckRepo.Create(ctx, check); err != nil {
		return nil, err
	}

	return &domain.ATSCheckResponse{
		ATSCheck:         check,
		AIAnalysisStatus: aiStatus,
	}, nil
}

func (s *atsCheckService) GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.ATSCheck, error) {
	check, err := s.atsCheckRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrATSCheckNotFound
		}
		return nil, err
	}

	if check.UserID != userID {
		return nil, ErrATSCheckUnauthorized
	}

	return check, nil
}

func (s *atsCheckService) GetByUserID(ctx context.Context, userID uuid.UUID, page, limit int) (*domain.PaginatedATSChecks, error) {
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

	total, err := s.atsCheckRepo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	checks, err := s.atsCheckRepo.FindByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	return &domain.PaginatedATSChecks{
		ATSChecks: checks,
		Pagination: domain.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *atsCheckService) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	check, err := s.atsCheckRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrATSCheckNotFound
		}
		return err
	}

	if check.UserID != userID {
		return ErrATSCheckUnauthorized
	}

	return s.atsCheckRepo.SoftDelete(ctx, id)
}

func (s *atsCheckService) analyzeFile(ctx context.Context, file *multipart.FileHeader) (*domain.ATSAnalysis, error) {
	result, err := s.genaiClient.GenerateFromFileWithSystemPrompt(
		ctx,
		file,
		atsFileAnalysisSystemPrompt,
		atsFileAnalysisUserPrompt,
	)
	if err != nil {
		return nil, err
	}

	cleaned := cleanJSONResponse(result)

	var analysis domain.ATSAnalysis
	if err := json.Unmarshal([]byte(cleaned), &analysis); err != nil {
		return nil, err
	}

	return &analysis, nil
}

func (s *atsCheckService) buildFallbackAnalysis() *domain.ATSAnalysis {
	return &domain.ATSAnalysis{
		OverallScore: 0,
		Verdict:      "AI analysis failed. Please try again later.",
		Sections: []domain.ATSSection{
			{Name: "Contact Information", Score: 0, MaxScore: 10, Feedback: "Could not analyze — AI unavailable"},
			{Name: "Professional Summary", Score: 0, MaxScore: 15, Feedback: "Could not analyze — AI unavailable"},
			{Name: "Work Experience", Score: 0, MaxScore: 30, Feedback: "Could not analyze — AI unavailable"},
			{Name: "Education", Score: 0, MaxScore: 10, Feedback: "Could not analyze — AI unavailable"},
			{Name: "Skills", Score: 0, MaxScore: 15, Feedback: "Could not analyze — AI unavailable"},
			{Name: "Achievements & Impact", Score: 0, MaxScore: 10, Feedback: "Could not analyze — AI unavailable"},
			{Name: "Formatting & ATS Compatibility", Score: 0, MaxScore: 10, Feedback: "Could not analyze — AI unavailable"},
		},
		KeywordAnalysis: domain.ATSKeywords{
			Found:   []string{},
			Missing: []string{},
			Tip:     "AI analysis failed. Please retry to get keyword analysis.",
		},
		Improvements: []domain.ATSImprovement{
			{Priority: "critical", Category: "System", Issue: "AI analysis failed", Suggestion: "Please upload your resume again and retry"},
		},
		DealBreakers: []string{"Unable to analyze — please retry"},
	}
}

func cleanJSONResponse(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw)
}
