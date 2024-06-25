package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/lib/pq"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/stakwork/sphinx-tribes/auth"
	"github.com/stakwork/sphinx-tribes/db"
	mocks "github.com/stakwork/sphinx-tribes/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateChannel(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	cHandler := NewChannelHandler(db.TestDB)

	createTestPersonAndTribe := func(pubKey, tribeUUID, tribeName string) (db.Person, db.Tribe) {
		person := db.Person{
			Uuid:         "person_chan_uuid",
			OwnerAlias:   "person_chan",
			UniqueName:   "person_chan",
			OwnerPubKey:  pubKey,
			PriceToMeet:  0,
			Description:  "This is test user chan",
			Unlisted:     false,
			Tags:         pq.StringArray{},
			GithubIssues: db.PropertyMap{},
			Extras:       db.PropertyMap{"coding_languages": "Lightning"},
		}
		db.TestDB.CreateOrEditPerson(person)

		tribe := db.Tribe{
			UUID:        tribeUUID,
			OwnerPubKey: person.OwnerPubKey,
			OwnerAlias:  person.OwnerAlias,
			Name:        tribeName,
			Unlisted:    false,
			UniqueName:  tribeName,
		}
		db.TestDB.CreateOrEditTribe(tribe)

		return person, tribe
	}

	t.Run("Should test that a user that is not authenticated cannot create a channel", func(t *testing.T) {
		_, tribe := createTestPersonAndTribe("person_chan_pubkey", "tribe_uuid", "New Tribe")

		requestBody := map[string]interface{}{
			"tribe_uuid": tribe.UUID,
			"name":       "Test Channel",
		}
		requestBodyBytes, err := json.Marshal(requestBody)
		assert.NoError(t, err)

		req, err := http.NewRequest("POST", "/channel", bytes.NewBuffer(requestBodyBytes))
		assert.NoError(t, err)
		rr := httptest.NewRecorder()

		cHandler.CreateChannel(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Should test that an authenticated user can create a channel", func(t *testing.T) {
		person, tribe := createTestPersonAndTribe("person_chan_pubkey", "tribe_uuid", "New Tribe")

		requestBody := map[string]interface{}{
			"tribe_uuid": tribe.UUID,
			"name":       "New Channel",
		}
		requestBodyBytes, err := json.Marshal(requestBody)
		assert.NoError(t, err)

		req, err := http.NewRequest("POST", "/channel", bytes.NewBuffer(requestBodyBytes))
		assert.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey))
		rr := httptest.NewRecorder()

		cHandler.CreateChannel(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		channels := db.TestDB.GetChannelsByTribe(tribe.UUID)
		assert.Equal(t, 1, len(channels))
		assert.Equal(t, "New Channel", channels[0].Name)
	})

	t.Run("Should test that a user cannot create a channel with a name that already exists", func(t *testing.T) {
		person, tribe := createTestPersonAndTribe("person_chan_pubkey", "tribe_uuid", "New Tribe")

		channel := db.Channel{
			TribeUUID: tribe.UUID,
			Name:      "Test Channel",
			Deleted:   false,
		}
		db.TestDB.CreateChannel(channel)

		requestBody := map[string]interface{}{
			"tribe_uuid": tribe.UUID,
			"name":       "Test Channel",
		}
		requestBodyBytes, err := json.Marshal(requestBody)
		assert.NoError(t, err)

		req, err := http.NewRequest("POST", "/channel", bytes.NewBuffer(requestBodyBytes))
		assert.NoError(t, err)
		req = req.WithContext(context.WithValue(req.Context(), auth.ContextKey, person.OwnerPubKey))
		rr := httptest.NewRecorder()

		cHandler.CreateChannel(rr, req)

		assert.Equal(t, http.StatusNotAcceptable, rr.Code)
	})
}

func TestDeleteChannel(t *testing.T) {
	ctx := context.WithValue(context.Background(), auth.ContextKey, "mock_pubkey")
	mockDb := mocks.NewDatabase(t)
	cHandler := NewChannelHandler(mockDb)

	// Mock data for testing
	mockPubKey := "mock_pubkey"
	mockChannelID := uint(1)

	t.Run("Should test that the owner of a channel can delete the channel", func(t *testing.T) {
		mockDb.On("GetChannel", mockChannelID).Return(db.Channel{ID: mockChannelID, TribeUUID: "mock_tribe_uuid"})
		mockDb.On("GetTribe", "mock_tribe_uuid").Return(db.Tribe{OwnerPubKey: mockPubKey})
		mockDb.On("UpdateChannel", mockChannelID, mock.Anything).Return(true)

		// Create and Serve request
		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(ctx, "DELETE", "/channel/1", nil)
		if err != nil {
			t.Fatal(err)
		}
		chiCtx := chi.NewRouteContext()
		chiCtx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

		handler := http.HandlerFunc(cHandler.DeleteChannel)
		handler.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Should test that non-channel owners cannot delete the channel, it should return a 401 error", func(t *testing.T) {
		mockPubKey := "other_pubkey"

		mockDb.ExpectedCalls = nil
		mockDb.On("GetChannel", mockChannelID).Return(db.Channel{ID: mockChannelID, TribeUUID: "mock_tribe_uuid"})
		mockDb.On("GetTribe", "mock_tribe_uuid").Return(db.Tribe{OwnerPubKey: mockPubKey})

		// Create and Serve request
		rr := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(ctx, "DELETE", "/channel/1", nil)
		if err != nil {
			t.Fatal(err)
		}
		chiCtx := chi.NewRouteContext()
		chiCtx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

		handler := http.HandlerFunc(cHandler.DeleteChannel)
		handler.ServeHTTP(rr, req)

		// Verify response
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}
