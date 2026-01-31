package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/raflytch/careerly-server/internal/domain"
	"github.com/raflytch/careerly-server/pkg/genai"

	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
)

var (
	ErrResumeNotFound = errors.New("resume not found")
	ErrUnauthorized   = errors.New("unauthorized access to resume")
)

const resumeSystemPrompt = `You are a professional resume writer and career coach. Your task is to transform casual, everyday language descriptions into professional, ATS-friendly content while maintaining accuracy and authenticity.

Guidelines:
1. Convert informal language to professional terminology
2. Use strong action verbs (e.g., "Led", "Developed", "Implemented", "Achieved")
3. Quantify achievements where possible
4. Keep descriptions concise but impactful
5. Maintain the original meaning and facts
6. Use industry-standard keywords for ATS optimization
7. Format experience descriptions as bullet-point worthy content
8. Ensure grammar and spelling are perfect

Respond ONLY with valid JSON in the exact same structure as the input, with the text content professionally rewritten. Do not add any explanation or markdown formatting.`

type resumeService struct {
	resumeRepo   domain.ResumeRepository
	quotaService domain.QuotaService
	genaiClient  *genai.Client
	cacheRepo    domain.CacheRepository
}

func NewResumeService(
	resumeRepo domain.ResumeRepository,
	quotaService domain.QuotaService,
	genaiClient *genai.Client,
	cacheRepo domain.CacheRepository,
) domain.ResumeService {
	return &resumeService{
		resumeRepo:   resumeRepo,
		quotaService: quotaService,
		genaiClient:  genaiClient,
		cacheRepo:    cacheRepo,
	}
}

func (s *resumeService) Create(ctx context.Context, userID uuid.UUID, req *domain.CreateResumeRequest) (*domain.ResumeResponse, error) {
	if err := s.quotaService.CheckAndIncrementUsage(ctx, userID, domain.FeatureResume); err != nil {
		return nil, err
	}

	content := domain.ResumeContent{
		PersonalInfo: req.PersonalInfo,
		Summary:      req.Summary,
		Experience:   req.Experience,
		Education:    req.Education,
		Skills:       req.Skills,
		Achievements: req.Achievements,
		Volunteer:    req.Volunteer,
		Languages:    req.Languages,
		Hobbies:      req.Hobbies,
	}

	aiStatus := "success"
	professionalContent, err := s.convertToProfessional(ctx, content)
	if err != nil {
		professionalContent = content
		if s.genaiClient == nil {
			aiStatus = "skipped_no_ai_client"
		} else {
			aiStatus = "failed_using_original"
		}
	}

	resume := &domain.Resume{
		ID:        uuid.New(),
		UserID:    userID,
		Title:     req.Title,
		Content:   professionalContent,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.resumeRepo.Create(ctx, resume); err != nil {
		return nil, err
	}

	return &domain.ResumeResponse{
		Resume:             resume,
		AIConversionStatus: aiStatus,
	}, nil
}

func (s *resumeService) GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.Resume, error) {
	resume, err := s.resumeRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrResumeNotFound
		}
		return nil, err
	}

	if resume.UserID != userID {
		return nil, ErrUnauthorized
	}

	return resume, nil
}

func (s *resumeService) GetByUserID(ctx context.Context, userID uuid.UUID, page, limit int) (*domain.PaginatedResumes, error) {
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

	total, err := s.resumeRepo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	resumes, err := s.resumeRepo.FindByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	return &domain.PaginatedResumes{
		Resumes: resumes,
		Pagination: domain.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *resumeService) Update(ctx context.Context, userID uuid.UUID, id uuid.UUID, req *domain.UpdateResumeRequest) (*domain.ResumeResponse, error) {
	resume, err := s.resumeRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrResumeNotFound
		}
		return nil, err
	}

	if resume.UserID != userID {
		return nil, ErrUnauthorized
	}

	if req.Title != nil {
		resume.Title = *req.Title
	}
	if req.PersonalInfo != nil {
		resume.Content.PersonalInfo = *req.PersonalInfo
	}
	if req.Summary != nil {
		resume.Content.Summary = *req.Summary
	}
	if req.Experience != nil {
		resume.Content.Experience = req.Experience
	}
	if req.Education != nil {
		resume.Content.Education = req.Education
	}
	if req.Skills != nil {
		resume.Content.Skills = req.Skills
	}
	if req.Achievements != nil {
		resume.Content.Achievements = req.Achievements
	}
	if req.Volunteer != nil {
		resume.Content.Volunteer = req.Volunteer
	}
	if req.Languages != nil {
		resume.Content.Languages = req.Languages
	}
	if req.Hobbies != nil {
		resume.Content.Hobbies = req.Hobbies
	}
	if req.IsActive != nil {
		resume.IsActive = *req.IsActive
	}

	aiStatus := "success"
	professionalContent, err := s.convertToProfessional(ctx, resume.Content)
	if err != nil {
		if s.genaiClient == nil {
			aiStatus = "skipped_no_ai_client"
		} else {
			aiStatus = "failed_using_original"
		}
	} else {
		resume.Content = professionalContent
	}

	resume.UpdatedAt = time.Now()

	if err := s.resumeRepo.Update(ctx, resume); err != nil {
		return nil, err
	}

	return &domain.ResumeResponse{
		Resume:             resume,
		AIConversionStatus: aiStatus,
	}, nil
}

func (s *resumeService) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	resume, err := s.resumeRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrResumeNotFound
		}
		return err
	}

	if resume.UserID != userID {
		return ErrUnauthorized
	}

	return s.resumeRepo.SoftDelete(ctx, id)
}

func (s *resumeService) GeneratePDF(ctx context.Context, userID uuid.UUID, id uuid.UUID) ([]byte, error) {
	resume, err := s.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	return s.generatePDFFromResume(resume)
}

func (s *resumeService) convertToProfessional(ctx context.Context, content domain.ResumeContent) (domain.ResumeContent, error) {
	if s.genaiClient == nil {
		return content, nil
	}

	contentJSON, err := json.Marshal(content)
	if err != nil {
		return content, err
	}

	result, err := s.genaiClient.GenerateJSONWithSystemPrompt(ctx, resumeSystemPrompt, string(contentJSON))
	if err != nil {
		return content, err
	}

	var professionalContent domain.ResumeContent
	if err := json.Unmarshal([]byte(result), &professionalContent); err != nil {
		return content, err
	}

	return professionalContent, nil
}

func (s *resumeService) generatePDFFromResume(resume *domain.Resume) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(0, 8, resume.Content.PersonalInfo.FullName)
	pdf.Ln(7)

	pdf.SetFont("Helvetica", "", 9)
	contactInfo := fmt.Sprintf("%s  |  %s  |  %s",
		resume.Content.PersonalInfo.Email,
		resume.Content.PersonalInfo.Phone,
		resume.Content.PersonalInfo.Location,
	)
	pdf.Cell(0, 5, contactInfo)
	pdf.Ln(5)

	links := ""
	if resume.Content.PersonalInfo.LinkedIn != "" {
		links += resume.Content.PersonalInfo.LinkedIn
	}
	if resume.Content.PersonalInfo.Portfolio != "" {
		if links != "" {
			links += "  |  "
		}
		links += resume.Content.PersonalInfo.Portfolio
	}
	if links != "" {
		pdf.Cell(0, 5, links)
		pdf.Ln(5)
	}

	pdf.Ln(4)

	if resume.Content.Summary != "" {
		s.addSection(pdf, "PROFESSIONAL SUMMARY")
		pdf.SetFont("Helvetica", "", 9)
		pdf.MultiCell(0, 4, resume.Content.Summary, "", "", false)
		pdf.Ln(3)
	}

	if len(resume.Content.Experience) > 0 {
		s.addSection(pdf, "WORK EXPERIENCE")
		for _, exp := range resume.Content.Experience {
			pdf.SetFont("Helvetica", "B", 10)
			pdf.Cell(0, 5, exp.Position)
			pdf.Ln(5)
			pdf.SetFont("Helvetica", "I", 9)
			location := ""
			if exp.Location != "" {
				location = " | " + exp.Location
			}
			pdf.Cell(0, 4, fmt.Sprintf("%s | %s - %s%s", exp.Company, exp.StartDate, exp.EndDate, location))
			pdf.Ln(5)
			pdf.SetFont("Helvetica", "", 9)
			s.addBulletPoints(pdf, exp.Description)
			pdf.Ln(2)
		}
		pdf.Ln(1)
	}

	if len(resume.Content.Education) > 0 {
		s.addSection(pdf, "EDUCATION")
		for _, edu := range resume.Content.Education {
			pdf.SetFont("Helvetica", "B", 10)
			pdf.Cell(0, 5, fmt.Sprintf("%s in %s", edu.Degree, edu.Field))
			pdf.Ln(5)
			pdf.SetFont("Helvetica", "I", 9)
			eduInfo := fmt.Sprintf("%s | %s - %s", edu.Institution, edu.StartDate, edu.EndDate)
			if edu.GPA != "" {
				eduInfo += fmt.Sprintf(" | GPA: %s", edu.GPA)
			}
			pdf.Cell(0, 4, eduInfo)
			pdf.Ln(5)
		}
		pdf.Ln(1)
	}

	if len(resume.Content.Skills) > 0 {
		s.addSection(pdf, "SKILLS")
		pdf.SetFont("Helvetica", "", 9)
		skillsText := ""
		for i, skill := range resume.Content.Skills {
			if i > 0 {
				skillsText += "  |  "
			}
			skillsText += skill
		}
		pdf.MultiCell(0, 4, skillsText, "", "", false)
		pdf.Ln(3)
	}

	if len(resume.Content.Achievements) > 0 {
		s.addSection(pdf, "ACHIEVEMENTS")
		pdf.SetFont("Helvetica", "", 9)
		for _, achievement := range resume.Content.Achievements {
			pdf.CellFormat(5, 4, "-", "", 0, "", false, 0, "")
			pdf.MultiCell(0, 4, achievement, "", "", false)
		}
		pdf.Ln(1)
	}

	if len(resume.Content.Volunteer) > 0 {
		s.addSection(pdf, "VOLUNTEER EXPERIENCE")
		for _, vol := range resume.Content.Volunteer {
			pdf.SetFont("Helvetica", "B", 10)
			pdf.Cell(0, 5, vol.Role)
			pdf.Ln(5)
			pdf.SetFont("Helvetica", "I", 9)
			pdf.Cell(0, 4, fmt.Sprintf("%s | %s - %s", vol.Organization, vol.StartDate, vol.EndDate))
			pdf.Ln(5)
			pdf.SetFont("Helvetica", "", 9)
			s.addBulletPoints(pdf, vol.Description)
			pdf.Ln(2)
		}
		pdf.Ln(1)
	}

	if len(resume.Content.Languages) > 0 {
		s.addSection(pdf, "LANGUAGES")
		pdf.SetFont("Helvetica", "", 9)
		langText := ""
		for i, lang := range resume.Content.Languages {
			if i > 0 {
				langText += "  |  "
			}
			langText += fmt.Sprintf("%s (%s)", lang.Name, lang.Proficiency)
		}
		pdf.Cell(0, 4, langText)
		pdf.Ln(4)
	}

	if len(resume.Content.Hobbies) > 0 {
		s.addSection(pdf, "HOBBIES & INTERESTS")
		pdf.SetFont("Helvetica", "", 9)
		hobbiesText := ""
		for i, hobby := range resume.Content.Hobbies {
			if i > 0 {
				hobbiesText += "  |  "
			}
			hobbiesText += hobby
		}
		pdf.Cell(0, 4, hobbiesText)
		pdf.Ln(4)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *resumeService) addSection(pdf *fpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 10)
	pdf.Cell(0, 6, title)
	pdf.Ln(6)
	pdf.SetDrawColor(100, 100, 100)
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(3)
}

func (s *resumeService) addBulletPoints(pdf *fpdf.Fpdf, text string) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimPrefix(line, "•")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pdf.CellFormat(5, 4, "-", "", 0, "", false, 0, "")
		pdf.MultiCell(0, 4, line, "", "", false)
	}
}
