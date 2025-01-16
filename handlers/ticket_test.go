package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/stakwork/sphinx-tribes/auth"
	"github.com/stakwork/sphinx-tribes/db"
	"github.com/stakwork/sphinx-tribes/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTicket(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	tHandler := NewTicketHandler(&http.Client{}, db.TestDB)

	person := db.Person{
		Uuid:        uuid.New().String(),
		OwnerAlias:  "test-alias",
		UniqueName:  "test-unique-name",
		OwnerPubKey: "test-pubkey",
		PriceToMeet: 0,
		Description: "test-description",
	}
	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        uuid.New().String(),
		Name:        "test-workspace" + uuid.New().String(),
		OwnerPubKey: person.OwnerPubKey,
		Github:      "https://github.com/test",
		Website:     "https://www.testwebsite.com",
		Description: "test-description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)

	featurePhase := db.FeaturePhase{
		Uuid:        uuid.New().String(),
		FeatureUuid: feature.Uuid,
		Name:        "test-phase",
		Priority:    0,
	}
	db.TestDB.CreateOrEditFeaturePhase(featurePhase)

	ticket := db.Tickets{
		UUID:        uuid.New(),
		FeatureUUID: feature.Uuid,
		PhaseUUID:   featurePhase.Uuid,
		Name:        "Test Ticket",
		Sequence:    1,
		Description: "Test Description",
		Status:      db.DraftTicket,
	}
	createdTicket, _ := db.TestDB.UpdateTicket(ticket)

	t.Run("should return 400 if UUID is empty", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.GetTicket)

		req, err := http.NewRequest(http.MethodGet, "/tickets/", nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should return 404 if ticket doesn't exist", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.GetTicket)

		nonExistentUUID := uuid.New()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", nonExistentUUID.String())
		req, err := http.NewRequest(http.MethodGet, "/tickets/"+nonExistentUUID.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("should return ticket if exists", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.GetTicket)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", createdTicket.UUID.String())
		req, err := http.NewRequest(http.MethodGet, "/tickets/"+createdTicket.UUID.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var returnedTicket db.Tickets
		err = json.Unmarshal(rr.Body.Bytes(), &returnedTicket)
		assert.NoError(t, err)
		assert.Equal(t, createdTicket.Name, returnedTicket.Name)
		assert.Equal(t, createdTicket.Description, returnedTicket.Description)
		assert.Equal(t, createdTicket.Status, returnedTicket.Status)
		assert.Equal(t, createdTicket.FeatureUUID, returnedTicket.FeatureUUID)
		assert.Equal(t, createdTicket.PhaseUUID, returnedTicket.PhaseUUID)
	})
}

func TestUpdateTicket(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	tHandler := NewTicketHandler(&http.Client{}, db.TestDB)

	person := db.Person{
		Uuid:        uuid.New().String(),
		OwnerAlias:  "test-alias",
		UniqueName:  "test-unique-name",
		OwnerPubKey: "test-pubkey",
		PriceToMeet: 0,
		Description: "test-description",
	}
	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        uuid.New().String(),
		Name:        "test-workspace" + uuid.New().String(),
		OwnerPubKey: person.OwnerPubKey,
		Github:      "https://github.com/test",
		Website:     "https://www.testwebsite.com",
		Description: "test-description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)

	featurePhase := db.FeaturePhase{
		Uuid:        uuid.New().String(),
		FeatureUuid: feature.Uuid,
		Name:        "test-phase",
		Priority:    0,
	}
	db.TestDB.CreateOrEditFeaturePhase(featurePhase)

	ticket := db.Tickets{
		UUID:        uuid.New(),
		FeatureUUID: feature.Uuid,
		PhaseUUID:   featurePhase.Uuid,
		Name:        "Test Ticket",
		Sequence:    1,
		Description: "Test Description",
		Status:      db.DraftTicket,
	}
	createdTicket, _ := db.TestDB.UpdateTicket(ticket)

	t.Run("should return 401 if no auth token", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.UpdateTicket)

		req, err := http.NewRequest(http.MethodPost, "/tickets/", nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return 400 if UUID is empty", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.UpdateTicket)

		req, err := http.NewRequest(http.MethodPost, "/tickets/", nil)
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey)
		req = req.WithContext(ctx)

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should return 400 if UUID is invalid", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.UpdateTicket)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", "invalid-uuid")

		req, err := http.NewRequest(http.MethodPost, "/tickets/invalid-uuid", bytes.NewReader([]byte("{}")))
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey)
		req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should return 400 if body is not valid json", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.UpdateTicket)

		invalidJson := []byte(`{"key": "value"`)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", createdTicket.UUID.String())
		req, err := http.NewRequest(http.MethodPost, "/tickets/"+createdTicket.UUID.String(), bytes.NewReader(invalidJson))
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey)
		req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should update ticket with only UUID and optional fields", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.UpdateTicket)

		updateTicket := createdTicket
		updateTicket.Description = "Updated Description"
		updateTicket.Status = db.ReadyTicket

		updateRequest := UpdateTicketRequest{
			Metadata: struct {
				Source string `json:"source"`
				ID     string `json:"id"`
			}{
				Source: "test-source",
				ID:     "test-id",
			},
			Ticket: &updateTicket,
		}

		requestBody, _ := json.Marshal(updateRequest)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", updateTicket.UUID.String())
		req, err := http.NewRequest(http.MethodPost, "/tickets/"+updateTicket.UUID.String(), bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey)
		req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var returnedTicket db.Tickets
		err = json.Unmarshal(rr.Body.Bytes(), &returnedTicket)
		assert.NoError(t, err)

		// Verify that only the provided fields were updated
		assert.Equal(t, updateTicket.Description, returnedTicket.Description)
		assert.Equal(t, updateTicket.Status, returnedTicket.Status)
		// Original fields should remain unchanged
		assert.Equal(t, createdTicket.FeatureUUID, returnedTicket.FeatureUUID)
		assert.Equal(t, createdTicket.PhaseUUID, returnedTicket.PhaseUUID)
		assert.Equal(t, createdTicket.Name, returnedTicket.Name)
	})

	t.Run("should update ticket successfully", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.UpdateTicket)

		updatedTicket := createdTicket
		updatedTicket.Name = "Updated Test Ticket"
		updatedTicket.Description = "Updated Description"
		updatedTicket.Status = db.CompletedTicket

		updateRequest := UpdateTicketRequest{
			Metadata: struct {
				Source string `json:"source"`
				ID     string `json:"id"`
			}{
				Source: "test-source",
				ID:     "test-id",
			},
			Ticket: &updatedTicket,
		}

		requestBody, _ := json.Marshal(updateRequest)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", createdTicket.UUID.String())

		req, err := http.NewRequest(http.MethodPost, "/tickets/"+createdTicket.UUID.String(), bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey)
		req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var returnedTicket db.Tickets
		err = json.Unmarshal(rr.Body.Bytes(), &returnedTicket)
		assert.NoError(t, err)
		assert.Equal(t, updatedTicket.Name, returnedTicket.Name)
		assert.Equal(t, updatedTicket.Description, returnedTicket.Description)
		assert.Equal(t, updatedTicket.Status, returnedTicket.Status)
		assert.Equal(t, updatedTicket.FeatureUUID, returnedTicket.FeatureUUID)
		assert.Equal(t, updatedTicket.PhaseUUID, returnedTicket.PhaseUUID)
	})

}

func TestDeleteTicket(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	tHandler := NewTicketHandler(&http.Client{}, db.TestDB)

	person := db.Person{
		Uuid:        uuid.New().String(),
		OwnerAlias:  "test-alias",
		UniqueName:  "test-unique-name",
		OwnerPubKey: "test-pubkey",
		PriceToMeet: 0,
		Description: "test-description",
	}
	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        uuid.New().String(),
		Name:        "test-workspace" + uuid.New().String(),
		OwnerPubKey: person.OwnerPubKey,
		Github:      "https://github.com/test",
		Website:     "https://www.testwebsite.com",
		Description: "test-description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)

	featurePhase := db.FeaturePhase{
		Uuid:        uuid.New().String(),
		FeatureUuid: feature.Uuid,
		Name:        "test-phase",
		Priority:    0,
	}
	db.TestDB.CreateOrEditFeaturePhase(featurePhase)

	ticket := db.Tickets{
		UUID:        uuid.New(),
		FeatureUUID: feature.Uuid,
		PhaseUUID:   featurePhase.Uuid,
		Name:        "Test Ticket",
		Sequence:    1,
		Description: "Test Description",
		Status:      db.DraftTicket,
	}
	createdTicket, _ := db.TestDB.UpdateTicket(ticket)

	t.Run("should return 401 if no auth token", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.DeleteTicket)

		req, err := http.NewRequest(http.MethodDelete, "/tickets/", nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return 400 if UUID is empty", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.DeleteTicket)

		req, err := http.NewRequest(http.MethodDelete, "/tickets/", nil)
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey)
		req = req.WithContext(ctx)

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should return 404 if ticket doesn't exist", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.DeleteTicket)

		nonExistentUUID := uuid.New()
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", nonExistentUUID.String())

		req, err := http.NewRequest(http.MethodDelete, "/tickets/"+nonExistentUUID.String(), nil)
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey)
		req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("should delete ticket successfully", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tHandler.DeleteTicket)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", createdTicket.UUID.String())

		req, err := http.NewRequest(http.MethodDelete, "/tickets/"+createdTicket.UUID.String(), nil)
		if err != nil {
			t.Fatal(err)
		}

		ctx := context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey)
		req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Verify ticket was deleted
		_, err = db.TestDB.GetTicket(createdTicket.UUID.String())
		assert.Error(t, err)
		assert.Equal(t, "ticket not found", err.Error())
	})
}

func TestTicketToBounty(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	tHandler := NewTicketHandler(&http.Client{}, db.TestDB)

	workspace := db.Workspace{
		Uuid:        uuid.New().String(),
		Name:        "test-workspace",
		OwnerPubKey: "test-pubkey",
	}
	_, err := db.TestDB.CreateOrEditWorkspace(workspace)
	require.NoError(t, err)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
	}
	_, err = db.TestDB.CreateOrEditFeature(feature)
	require.NoError(t, err)

	phase := db.FeaturePhase{
		Uuid:        uuid.New().String(),
		FeatureUuid: feature.Uuid,
		Name:        "test-phase",
	}
	_, err = db.TestDB.CreateOrEditFeaturePhase(phase)
	require.NoError(t, err)

	ticket := db.Tickets{
		UUID:        uuid.New(),
		FeatureUUID: feature.Uuid,
		PhaseUUID:   phase.Uuid,
		Name:        "Test Ticket",
		Description: "Test Description",
		Status:      db.DraftTicket,
	}
	createdTicket, err := db.TestDB.UpdateTicket(ticket)
	require.NoError(t, err)

	tests := []struct {
		name     string
		ticket   string
		auth     string
		wantCode int
		validate func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:     "unauthorized - no auth token",
			ticket:   createdTicket.UUID.String(),
			auth:     "",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "bad request - missing ticket UUID",
			ticket:   "",
			auth:     workspace.OwnerPubKey,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "not found - ticket doesn't exist",
			ticket:   uuid.New().String(),
			auth:     workspace.OwnerPubKey,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "success - creates bounty from ticket",
			ticket:   createdTicket.UUID.String(),
			auth:     workspace.OwnerPubKey,
			wantCode: http.StatusCreated,
			validate: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp CreateBountyResponse
				require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

				assert.True(t, resp.Success)
				assert.NotZero(t, resp.BountyID)
				assert.Equal(t, "Bounty created successfully", resp.Message)

				bounty := db.TestDB.GetBounty(resp.BountyID)

				assert.Equal(t, createdTicket.Name, bounty.Title)
				assert.Equal(t, createdTicket.Description, bounty.Description)
				assert.Equal(t, createdTicket.PhaseUUID, bounty.PhaseUuid)
				assert.Equal(t, "freelance_job_request", bounty.Type)
				assert.Equal(t, uint(21), bounty.Price)
				assert.True(t, bounty.Show)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/tickets/bounty", nil)

			if tt.ticket != "" {
				rctx := chi.NewRouteContext()
				rctx.URLParams.Add("ticket_uuid", tt.ticket)
				req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			}

			if tt.auth != "" {
				req = req.WithContext(context.WithValue(req.Context(), auth.ContextKey, tt.auth))
			}

			tHandler.TicketToBounty(rr, req)

			assert.Equal(t, tt.wantCode, rr.Code)
			if tt.validate != nil {
				tt.validate(t, rr)
			}
		})
	}
}

func TestTicketToBountyConversionAndEditing(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	tHandler := NewTicketHandler(&http.Client{}, db.TestDB)
	bHandler := NewBountyHandler(&http.Client{}, db.TestDB)

	person := db.Person{
		Uuid:        uuid.New().String(),
		OwnerAlias:  "test-alias",
		UniqueName:  "test-unique-name",
		OwnerPubKey: "test-pubkey",
		PriceToMeet: 0,
		Description: "test-description",
	}
	_, err := db.TestDB.CreateOrEditPerson(person)
	require.NoError(t, err)

	workspaceName := "test-workspace-" + uuid.New().String()
	workspace := db.Workspace{
		Uuid:        uuid.New().String(),
		Name:        workspaceName,
		OwnerPubKey: person.OwnerPubKey,
		Github:      "https://github.com/test",
		Website:     "https://www.testwebsite.com",
		Description: "test-description",
	}
	_, err = db.TestDB.CreateOrEditWorkspace(workspace)
	require.NoError(t, err)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
		CreatedBy:     person.OwnerPubKey,
	}
	_, err = db.TestDB.CreateOrEditFeature(feature)
	require.NoError(t, err)

	phase := db.FeaturePhase{
		Uuid:        uuid.New().String(),
		FeatureUuid: feature.Uuid,
		Name:        "test-phase",
		Priority:    0,
	}
	_, err = db.TestDB.CreateOrEditFeaturePhase(phase)
	require.NoError(t, err)

	ticketUUID := uuid.New()
	ticket := db.Tickets{
		UUID:        ticketUUID,
		FeatureUUID: feature.Uuid,
		PhaseUUID:   phase.Uuid,
		Name:        "Test Ticket",
		Description: "Test Description",
		Status:      db.DraftTicket,
		AuthorID:    &person.OwnerPubKey,
		Features:    feature,
	}
	createdTicket, err := db.TestDB.CreateOrEditTicket(&ticket)
	require.NoError(t, err)

	t.Run("should create bounty from ticket and allow editing", func(t *testing.T) {
		// Step 1: Create bounty from ticket
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/tickets/"+ticketUUID.String()+"/bounty", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("ticket_uuid", createdTicket.UUID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		// Add auth context
		req = req.WithContext(context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey))

		tHandler.TicketToBounty(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var resp CreateBountyResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.NotZero(t, resp.BountyID)

		// Step 2: Edit bounty title
		updatedTitle := "Updated Bounty Title"
		now := time.Now()
		bounty := db.NewBounty{
			ID:          resp.BountyID,
			Title:       updatedTitle,
			Description: ticket.Description,
			OwnerID:     person.OwnerPubKey,
			Type:        "Other",
			PhaseUuid:   phase.Uuid,
			Show:        true,
			Price:       21,
			Created:     now.Unix(),
			Updated:     &now,
		}

		rr = httptest.NewRecorder()
		requestBody, err := json.Marshal(bounty)
		require.NoError(t, err)

		req = httptest.NewRequest(http.MethodPost, "/gobounties", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey))

		bHandler.CreateOrEditBounty(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		// Verify title was updated
		var updatedBounty db.NewBounty
		err = json.NewDecoder(rr.Body).Decode(&updatedBounty)
		require.NoError(t, err)
		assert.Equal(t, updatedTitle, updatedBounty.Title)

		// Step 3: Assign user to bounty
		assignee := db.Person{
			Uuid:        uuid.New().String(),
			OwnerAlias:  "assignee-alias",
			UniqueName:  "assignee-unique-name",
			OwnerPubKey: "assignee-pubkey",
		}
		_, err = db.TestDB.CreateOrEditPerson(assignee)
		require.NoError(t, err)

		bounty.Assignee = assignee.OwnerPubKey
		requestBody, err = json.Marshal(bounty)
		require.NoError(t, err)

		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/gobounties", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey))

		bHandler.CreateOrEditBounty(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		// Verify assignee was updated
		err = json.NewDecoder(rr.Body).Decode(&updatedBounty)
		require.NoError(t, err)
		assert.Equal(t, assignee.OwnerPubKey, updatedBounty.Assignee)
	})
}

func TestProcessTicketReview(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	tHandler := NewTicketHandler(&http.Client{}, db.TestDB)

	person := db.Person{
		Uuid:        uuid.New().String(),
		OwnerAlias:  "test-alias",
		UniqueName:  "test-unique-name",
		OwnerPubKey: "test-pubkey",
		PriceToMeet: 0,
		Description: "test-description",
	}
	_, err := db.TestDB.CreateOrEditPerson(person)
	require.NoError(t, err)

	workspace := db.Workspace{
		Uuid:        uuid.New().String(),
		Name:        "test-workspace" + uuid.New().String(),
		OwnerPubKey: person.OwnerPubKey,
		Github:      "https://github.com/test",
		Website:     "https://www.testwebsite.com",
		Description: "test-description",
	}
	_, err = db.TestDB.CreateOrEditWorkspace(workspace)
	require.NoError(t, err)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
	}
	createdFeature, err := db.TestDB.CreateOrEditFeature(feature)
	require.NoError(t, err)

	featurePhase := db.FeaturePhase{
		Uuid:        uuid.New().String(),
		FeatureUuid: feature.Uuid,
		Name:        "test-phase",
		Priority:    0,
	}
	createdPhase, err := db.TestDB.CreateOrEditFeaturePhase(featurePhase)
	require.NoError(t, err)

	ticket := db.Tickets{
		UUID:        uuid.New(),
		FeatureUUID: createdFeature.Uuid,
		PhaseUUID:   createdPhase.Uuid,
		Name:        "Test Ticket",
		Sequence:    1,
		Description: "Test Description",
		Status:      db.DraftTicket,
		Version:     0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	createdTicket, err := db.TestDB.CreateOrEditTicket(&ticket)
	require.NoError(t, err)

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectedBody   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "Valid Request",
			requestBody: utils.TicketReviewRequest{
				Value: struct {
					FeatureUUID       string `json:"featureUUID"`
					PhaseUUID         string `json:"phaseUUID"`
					TicketUUID        string `json:"ticketUUID" validate:"required"`
					TicketDescription string `json:"ticketDescription" validate:"required"`
					TicketName        string `json:"ticketName,omitempty"`
				}{
					FeatureUUID:       createdFeature.Uuid,
					PhaseUUID:         createdPhase.Uuid,
					TicketUUID:        createdTicket.UUID.String(),
					TicketDescription: "Updated Description",
				},
				SourceWebsocket: "test-websocket",
			},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, rr *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, rr.Code)

				var response struct {
					Ticket db.Tickets `json:"ticket"`
				}
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)

				returnedTicket := response.Ticket
				assert.Equal(t, "Updated Description", returnedTicket.Description)
			},
		},
		{
			name:           "Empty Request Body",
			requestBody:    nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, rr *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusBadRequest, rr.Code)
			},
		},
		{
			name:           "Malformed JSON",
			requestBody:    "{invalid-json}",
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, rr *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusBadRequest, rr.Code)
				assert.Contains(t, rr.Body.String(), "Error parsing request body")
			},
		},
		{
			name: "Missing Required Fields",
			requestBody: utils.TicketReviewRequest{
				Value: struct {
					FeatureUUID       string `json:"featureUUID"`
					PhaseUUID         string `json:"phaseUUID"`
					TicketUUID        string `json:"ticketUUID" validate:"required"`
					TicketDescription string `json:"ticketDescription" validate:"required"`
					TicketName        string `json:"ticketName,omitempty"`
				}{
					FeatureUUID:       createdFeature.Uuid,
					PhaseUUID:         createdPhase.Uuid,
					TicketDescription: "Updated Description",
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, rr *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusBadRequest, rr.Code)
				assert.Contains(t, rr.Body.String(), "ticketUUID is required")
			},
		},
		{
			name: "Non-existent TicketUUID",
			requestBody: utils.TicketReviewRequest{
				Value: struct {
					FeatureUUID       string `json:"featureUUID"`
					PhaseUUID         string `json:"phaseUUID"`
					TicketUUID        string `json:"ticketUUID" validate:"required"`
					TicketDescription string `json:"ticketDescription" validate:"required"`
					TicketName        string `json:"ticketName,omitempty"`
				}{
					TicketUUID:        "non-existent-uuid",
					TicketDescription: "New description",
				},
			},
			expectedStatus: http.StatusNotFound,
			expectedBody: func(t *testing.T, rr *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusNotFound, rr.Code)
				var response map[string]interface{}
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "Ticket not found", response["error"])
			},
		},
		{
			name: "Websocket Error",
			requestBody: utils.TicketReviewRequest{
				Value: struct {
					FeatureUUID       string `json:"featureUUID"`
					PhaseUUID         string `json:"phaseUUID"`
					TicketUUID        string `json:"ticketUUID" validate:"required"`
					TicketDescription string `json:"ticketDescription" validate:"required"`
					TicketName        string `json:"ticketName,omitempty"`
				}{
					TicketUUID:        createdTicket.UUID.String(),
					TicketDescription: "New description",
				},
				SourceWebsocket: "source-session-id",
			},

			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, rr *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, rr.Code)

				var response map[string]interface{}
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "client not found: source-session-id", response["websocket_error"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()

			r := chi.NewRouter()
			r.Mount("/bounties/ticket", func() http.Handler {
				r := chi.NewRouter()
				r.Post("/review", tHandler.ProcessTicketReview)
				return r
			}())

			var req *http.Request
			if tt.requestBody != nil {
				requestBody, _ := json.Marshal(tt.requestBody)
				req = httptest.NewRequest(http.MethodPost, "/bounties/ticket/review", bytes.NewBuffer(requestBody))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(http.MethodPost, "/bounties/ticket/review", nil)
			}

			r.ServeHTTP(rr, req)
			assert.Equal(t, tt.expectedStatus, rr.Code)
			tt.expectedBody(t, rr)
		})
	}
}

func TestGetTicketsByGroup(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	tHandler := NewTicketHandler(&http.Client{}, db.TestDB)

	person := db.Person{
		Uuid:        uuid.New().String(),
		OwnerAlias:  "test-alias",
		UniqueName:  "test-unique-name",
		OwnerPubKey: "test-pubkey",
		PriceToMeet: 0,
		Description: "test-description",
	}
	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        uuid.New().String(),
		Name:        "test-workspace" + uuid.New().String(),
		OwnerPubKey: person.OwnerPubKey,
		Github:      "https://github.com/test",
		Website:     "https://www.testwebsite.com",
		Description: "test-description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
	}
	createdFeature, err := db.TestDB.CreateOrEditFeature(feature)
	require.NoError(t, err)

	phase := db.FeaturePhase{
		Uuid:        uuid.New().String(),
		FeatureUuid: createdFeature.Uuid,
		Name:        "test-phase",
		Priority:    0,
	}
	createdPhase, err := db.TestDB.CreateOrEditFeaturePhase(phase)
	require.NoError(t, err)

	groupUUID := uuid.New()
	ticket := db.Tickets{
		UUID:        uuid.New(),
		TicketGroup: &groupUUID,
		FeatureUUID: createdFeature.Uuid,
		PhaseUUID:   createdPhase.Uuid,
		Name:        "Test Ticket",
		Description: "Test Description",
		Status:      db.DraftTicket,
		Version:     1,
	}
	createdTicket, err := db.TestDB.CreateOrEditTicket(&ticket)
	require.NoError(t, err, "Failed to create test ticket")

	tests := []struct {
		name       string
		groupUUID  string
		auth       string
		wantCode   int
		wantTicket bool
	}{
		{
			name:       "success",
			groupUUID:  groupUUID.String(),
			auth:       person.OwnerPubKey,
			wantCode:   http.StatusOK,
			wantTicket: true,
		},
		{
			name:      "unauthorized - no auth token",
			groupUUID: groupUUID.String(),
			auth:      "",
			wantCode:  http.StatusUnauthorized,
		},
		{
			name:      "bad request - invalid UUID",
			groupUUID: "invalid-uuid",
			auth:      person.OwnerPubKey,
			wantCode:  http.StatusBadRequest,
		},
		{
			name:       "not found - group doesn't exist",
			groupUUID:  uuid.New().String(),
			auth:       person.OwnerPubKey,
			wantCode:   http.StatusOK,
			wantTicket: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/tickets/group/"+tt.groupUUID, nil)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("group_uuid", tt.groupUUID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			if tt.auth != "" {
				req = req.WithContext(context.WithValue(req.Context(), auth.ContextKey, tt.auth))
			}

			tHandler.GetTicketsByGroup(rr, req)

			assert.Equal(t, tt.wantCode, rr.Code)

			if tt.wantTicket {
				var tickets []db.Tickets
				err := json.NewDecoder(rr.Body).Decode(&tickets)
				require.NoError(t, err)
				require.NotEmpty(t, tickets, "Expected non-empty tickets array")
				assert.Equal(t, createdTicket.UUID, tickets[0].UUID)
			}
		})
	}
}
