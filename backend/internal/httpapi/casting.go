package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ravshann/predlozhka/backend/internal/domain"
)

const maxCastingUploadBytes = 8 << 20

type castingStore interface {
	HasCastingAccess(context.Context, string) (bool, error)
	ListCastingReviewers(context.Context) ([]domain.User, error)
	AssignCastingReviewer(context.Context, string, string) (domain.User, error)
	RemoveCastingReviewer(context.Context, string, string) error
	CastingParticipants(context.Context) (json.RawMessage, error)
	ReplaceCastingParticipants(context.Context, json.RawMessage, string) error
	CastingScores(context.Context, string) (map[int]float64, error)
	SetCastingScore(context.Context, string, int, float64) error
}

type castingField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type castingSection struct {
	Title  string         `json:"title"`
	Fields []castingField `json:"fields"`
}

type castingVideo struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Text  string `json:"text,omitempty"`
}

type castingParticipant struct {
	ID       int              `json:"id"`
	FullName string           `json:"full_name"`
	Nickname string           `json:"nickname"`
	Twitch   string           `json:"twitch"`
	Age      string           `json:"age"`
	City     string           `json:"city"`
	Height   string           `json:"height"`
	Weight   string           `json:"weight"`
	Sections []castingSection `json:"sections"`
	Videos   []castingVideo   `json:"videos"`
}

func visibleCastingParticipants(participants []castingParticipant) []castingParticipant {
	result := make([]castingParticipant, 0, len(participants))
	for _, participant := range participants {
		copyParticipant := participant
		copyParticipant.Sections = make([]castingSection, 0, len(participant.Sections))
		for _, section := range participant.Sections {
			copySection := section
			copySection.Fields = make([]castingField, 0, len(section.Fields))
			for _, item := range section.Fields {
				if item.Label == "Фамилия" || item.Label == "Отчество" {
					continue
				}
				if item.Label == "Имя" {
					copyParticipant.FullName = item.Value
				}
				copySection.Fields = append(copySection.Fields, item)
			}
			copyParticipant.Sections = append(copyParticipant.Sections, copySection)
		}
		result = append(result, copyParticipant)
	}
	return result
}

func (s *Server) loadCastingParticipants(ctx context.Context, database castingStore) ([]castingParticipant, error) {
	payload, err := database.CastingParticipants(ctx)
	if err != nil {
		return nil, err
	}
	var participants []castingParticipant
	if len(payload) == 0 {
		return participants, nil
	}
	if err := json.Unmarshal(payload, &participants); err != nil {
		return nil, err
	}
	return participants, nil
}

func (s *Server) castingAccess(ctx context.Context, actor *actorContext) (bool, error) {
	if actor == nil {
		return false, nil
	}
	if actor.User.Role == domain.RoleOwner {
		return true, nil
	}
	database, ok := s.store.(castingStore)
	if !ok {
		return false, nil
	}
	return database.HasCastingAccess(ctx, actor.User.ID)
}

func (s *Server) requireCasting(w http.ResponseWriter, r *http.Request) *actorContext {
	actor, err := s.actor(r)
	if err != nil || actor == nil {
		writeError(w, http.StatusUnauthorized, "authentication_required", "Требуется авторизация")
		return nil
	}
	allowed, err := s.castingAccess(r.Context(), actor)
	if err != nil {
		s.internalError(w, err)
		return nil
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "casting_access_required", "Доступ к кастингу не выдан")
		return nil
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && s.require(w, r, "view_profile") == nil {
		return nil
	}
	return actor
}

func (s *Server) casting(w http.ResponseWriter, r *http.Request) {
	actor := s.requireCasting(w, r)
	if actor == nil {
		return
	}
	database, ok := s.store.(castingStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "casting_unavailable", "Кастинг временно недоступен")
		return
	}
	participants, err := s.loadCastingParticipants(r.Context(), database)
	if err != nil {
		s.internalError(w, err)
		return
	}
	scores, err := database.CastingScores(r.Context(), actor.User.ID)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"participants": visibleCastingParticipants(participants), "scores": scores}})
}

func (s *Server) setCastingScore(w http.ResponseWriter, r *http.Request) {
	actor := s.requireCasting(w, r)
	if actor == nil {
		return
	}
	database, ok := s.store.(castingStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "casting_unavailable", "Кастинг временно недоступен")
		return
	}
	participantID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || participantID < 1 {
		writeError(w, http.StatusBadRequest, "invalid_participant", "Участник не найден")
		return
	}
	participants, err := s.loadCastingParticipants(r.Context(), database)
	if err != nil {
		s.internalError(w, err)
		return
	}
	found := false
	for _, participant := range participants {
		if participant.ID == participantID {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusBadRequest, "invalid_participant", "Участник не найден")
		return
	}
	var input struct {
		Score float64 `json:"score"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Score < 1 || input.Score > 10 || input.Score*2 != float64(int(input.Score*2)) {
		writeError(w, http.StatusBadRequest, "invalid_score", "Оценка должна быть от 1 до 10 с шагом 0,5")
		return
	}
	if err := database.SetCastingScore(r.Context(), actor.User.ID, participantID, input.Score); err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"participant_id": participantID, "score": input.Score}})
}

func (s *Server) replaceCastingParticipants(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "manage")
	if actor == nil {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCastingUploadBytes)
	var input struct {
		Participants []castingParticipant `json:"participants"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if len(input.Participants) == 0 || len(input.Participants) > 2000 {
		writeError(w, http.StatusBadRequest, "invalid_casting_file", "В файле должно быть от 1 до 2000 участников")
		return
	}
	for index := range input.Participants {
		participant := &input.Participants[index]
		participant.ID = index + 1
		participant.FullName = strings.TrimSpace(participant.FullName)
		participant.City = strings.TrimSpace(participant.City)
		participant.Nickname = strings.TrimSpace(participant.Nickname)
		participant.Twitch = strings.TrimSpace(participant.Twitch)
		if participant.FullName == "" {
			writeError(w, http.StatusBadRequest, "invalid_casting_file", "У каждого участника должно быть указано имя")
			return
		}
		if len(participant.Sections) != 5 || len(participant.Videos) != 3 {
			writeError(w, http.StatusBadRequest, "invalid_casting_file", "Структура файла не соответствует анкете кастинга")
			return
		}
	}
	payload, err := json.Marshal(input.Participants)
	if err != nil {
		s.internalError(w, err)
		return
	}
	database, ok := s.store.(castingStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "casting_unavailable", "Кастинг временно недоступен")
		return
	}
	if err := database.ReplaceCastingParticipants(r.Context(), payload, actor.User.ID); err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]int{"count": len(input.Participants)}})
}

func (s *Server) listCastingReviewers(w http.ResponseWriter, r *http.Request) {
	if s.require(w, r, "manage") == nil {
		return
	}
	database, ok := s.store.(castingStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "casting_unavailable", "Кастинг временно недоступен")
		return
	}
	users, err := database.ListCastingReviewers(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": users})
}

func (s *Server) assignCastingReviewer(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "manage")
	if actor == nil {
		return
	}
	var input struct {
		TwitchLogin string `json:"twitch_login"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	login := strings.TrimPrefix(strings.TrimSpace(input.TwitchLogin), "@")
	if login == "" {
		writeError(w, http.StatusBadRequest, "invalid_twitch_login", "Укажите Twitch-ник")
		return
	}
	database, ok := s.store.(castingStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "casting_unavailable", "Кастинг временно недоступен")
		return
	}
	user, err := database.AssignCastingReviewer(r.Context(), login, actor.User.ID)
	if err != nil {
		s.storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": user})
}

func (s *Server) removeCastingReviewer(w http.ResponseWriter, r *http.Request) {
	actor := s.require(w, r, "manage")
	if actor == nil {
		return
	}
	database, ok := s.store.(castingStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "casting_unavailable", "Кастинг временно недоступен")
		return
	}
	if err := database.RemoveCastingReviewer(r.Context(), r.PathValue("id"), actor.User.ID); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
