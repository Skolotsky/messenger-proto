package conversations

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
)

const maxBody = 8 << 20

// Routes adds the people door's actions and questions, and the seed session, to
// mux. The live connection (T-053) and the agent door (T-054) add their own.
func (s *Store) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /orgs/{org}/people/actions", s.door(s.handleAct))
	mux.HandleFunc("GET /orgs/{org}/people/{person}/start", s.door(s.handleStart))
	mux.HandleFunc("GET /orgs/{org}/conversations/{conversation}/messages", s.door(s.handleMessages))
	mux.HandleFunc("GET /orgs/{org}/suggest", s.door(s.handleSuggest))
	mux.HandleFunc("GET /orgs/{org}/people", s.handlePeople)
	mux.HandleFunc("POST /orgs/{org}/seed/open", s.handleSeedOpen)
	mux.HandleFunc("POST /orgs/{org}/seed/{session}", s.handleSeedBatch)
	mux.HandleFunc("POST /orgs/{org}/seed/{session}/seal", s.handleSeedSeal)
	mux.HandleFunc("POST /orgs/{org}/retire", s.handleRetire)
	mux.HandleFunc("DELETE /orgs/{org}", s.handleDelete)
	s.agentRoutes(mux)
}

func send(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func orgOf(w http.ResponseWriter, r *http.Request) (ids.Org, bool) {
	org, err := ids.ParseOrg(r.PathValue("org"))
	if err != nil {
		send(w, http.StatusNotFound, contract.Refusal{Refused: "unknown organization"})
		return "", false
	}
	return org, true
}

// fail answers an error: a refusal when the caller named something that is not
// there, otherwise a server error, on which a screen keeps the action waiting.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var seed SeedRefused
	switch {
	case errors.As(err, &seed):
		send(w, http.StatusConflict, contract.Refusal{Refused: seed.Reason})
	case errors.Is(err, ErrOrganizationGone):
		WriteGone(w)
	case errors.Is(err, ErrUnknownOrg):
		send(w, http.StatusNotFound, contract.Refusal{Refused: "unknown organization"})
	case errors.Is(err, ErrNotFound):
		send(w, http.StatusNotFound, contract.Refusal{Refused: "not found"})
	default:
		slog.Error("request failed", "path", r.URL.Path, "err", err)
		send(w, http.StatusInternalServerError, map[string]string{"error": "try again"})
	}
}

func (s *Store) handleAct(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		send(w, http.StatusConflict, contract.Refusal{Refused: refusedUnreadable})
		return
	}
	status, answer, err := s.Act(r.Context(), org, raw)
	if err != nil {
		fail(w, r, err)
		return
	}
	send(w, status, answer)
}

func (s *Store) handleStart(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	answer, err := s.Start(r.Context(), org, r.PathValue("person"))
	if err != nil {
		fail(w, r, err)
		return
	}
	send(w, http.StatusOK, answer)
}

func (s *Store) handleMessages(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	v := r.URL.Query()
	q := MessagesQuery{Conversation: r.PathValue("conversation"), As: v.Get("as")}
	if t := v.Get("thread"); t != "" {
		q.Thread = &t
	}
	for name, into := range map[string]**int64{"around": &q.Around, "before": &q.Before} {
		if text := v.Get(name); text != "" {
			n, err := strconv.ParseInt(text, 10, 64)
			if err != nil {
				send(w, http.StatusBadRequest, contract.Refusal{Refused: "bad " + name})
				return
			}
			*into = &n
		}
	}
	if text := v.Get("limit"); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil || n <= 0 {
			send(w, http.StatusBadRequest, contract.Refusal{Refused: "bad limit"})
			return
		}
		q.Limit = n
	}
	answer, err := s.Messages(r.Context(), org, q)
	if err != nil {
		fail(w, r, err)
		return
	}
	send(w, http.StatusOK, answer)
}

func (s *Store) handleSuggest(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	answer, err := s.Suggest(r.Context(), org, r.URL.Query().Get("q"), r.URL.Query().Get("as"))
	if err != nil {
		fail(w, r, err)
		return
	}
	send(w, http.StatusOK, answer)
}

func (s *Store) handlePeople(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	answer, err := s.People(r.Context(), org)
	if err != nil {
		fail(w, r, err)
		return
	}
	send(w, http.StatusOK, answer)
}

func (s *Store) handleSeedOpen(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	var in contract.SeedOpenRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&in); err != nil {
			send(w, http.StatusConflict, contract.Refusal{Refused: refusedUnreadable})
			return
		}
	}
	name := ""
	if in.Name != nil {
		name = *in.Name
	}
	session, err := s.SeedOpenNamed(r.Context(), org, name)
	if err != nil {
		fail(w, r, err)
		return
	}
	send(w, http.StatusOK, contract.SeedOpened{Session: session})
}

func (s *Store) handleSeedBatch(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	var batch contract.SeedBatch
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&batch); err != nil {
		send(w, http.StatusConflict, contract.Refusal{Refused: refusedUnreadable})
		return
	}
	through, err := s.SeedBatch(r.Context(), org, r.PathValue("session"), batch)
	if err != nil {
		fail(w, r, err)
		return
	}
	send(w, http.StatusOK, contract.SeedThrough{Through: through})
}

func (s *Store) handleSeedSeal(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	through, err := s.SeedSeal(r.Context(), org, r.PathValue("session"))
	if err != nil {
		fail(w, r, err)
		return
	}
	send(w, http.StatusOK, contract.SeedThrough{Through: through})
}
