package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stakwork/sphinx-tribes/auth"
	"github.com/stakwork/sphinx-tribes/db"
	"github.com/stretchr/testify/assert"
)

func TestCreateOrEditFeatures(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

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
	workspace = db.TestDB.GetWorkspaceByUuid(workspace.Uuid)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
	}

	t.Run("should return 401 error if not authorized", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditFeatures)

		requestBody, _ := json.Marshal(feature)
		req, err := http.NewRequest(http.MethodPost, "/features", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return 406 error if body is not a valid json", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditFeatures)

		invalidJson := []byte(`{"key": "value"`)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/features", bytes.NewReader(invalidJson))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotAcceptable, rr.Code)
	})

	t.Run("should return 401 error if workspace UUID does not exist", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditFeatures)

		feature.WorkspaceUuid = "non-existent-uuid"
		requestBody, _ := json.Marshal(feature)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/features", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should successfully add feature if request is valid", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditFeatures)

		feature.WorkspaceUuid = workspace.Uuid
		requestBody, _ := json.Marshal(feature)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/features", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		createdFeature := db.TestDB.GetFeatureByUuid(feature.Uuid)
		assert.Equal(t, feature.Name, createdFeature.Name)
		assert.Equal(t, feature.Url, createdFeature.Url)
		assert.Equal(t, feature.Priority, createdFeature.Priority)
	})
}

func TestDeleteFeature(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)
	oHandler := NewFeatureHandler(db.TestDB)

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
	workspace = db.TestDB.GetWorkspaceByUuid(workspace.Uuid)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)
	feature = db.TestDB.GetFeatureByUuid(feature.Uuid)

	ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)

	t.Run("should return error if not authorized", func(t *testing.T) {
		featureUUID := feature.Uuid
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(oHandler.DeleteFeature)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", featureUUID)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodDelete, "/features/"+featureUUID, nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should delete feature on successful delete", func(t *testing.T) {
		featureUUID := feature.Uuid

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(oHandler.DeleteFeature)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", featureUUID)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodDelete, "/features/"+featureUUID, nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		deletedFeature := db.TestDB.GetFeatureByUuid(featureUUID)
		assert.Equal(t, db.WorkspaceFeatures{}, deletedFeature)
	})
}

func TestGetWorkspaceFeaturesCount(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	oHandler := NewFeatureHandler(db.TestDB)

	person := db.Person{
		Uuid:        "uuid",
		OwnerAlias:  "alias",
		UniqueName:  "unique_name",
		OwnerPubKey: "pubkey",
		PriceToMeet: 0,
		Description: "description",
	}
	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        "workspace_uuid",
		Name:        "workspace_name",
		OwnerPubKey: "person.OwnerPubkey",
		Github:      "gtihub",
		Website:     "website",
		Description: "description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	feature := db.WorkspaceFeatures{
		Uuid:          "feature_uuid",
		WorkspaceUuid: workspace.Uuid,
		Name:          "feature_name",
		Url:           "feature_url",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)

	ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)

	t.Run("Should test that it throws a 401 error if a user is not authorized", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", workspace.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/workspace/count/"+workspace.Uuid, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(oHandler.GetWorkspaceFeaturesCount).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Should test that the features count returned from the API response for the workspace is equal to the number of features created for the workspace", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", workspace.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodGet, "/workspace/count/"+workspace.Uuid, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(oHandler.GetWorkspaceFeaturesCount).ServeHTTP(rr, req)

		var returnedWorkspaceFeatures int64
		err = json.Unmarshal(rr.Body.Bytes(), &returnedWorkspaceFeatures)
		assert.NoError(t, err)

		featureCount := db.TestDB.GetWorkspaceFeaturesCount(workspace.Uuid)

		assert.Equal(t, returnedWorkspaceFeatures, featureCount)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestGetFeatureByUuid(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

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
	workspace = db.TestDB.GetWorkspaceByUuid(workspace.Uuid)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)
	feature = db.TestDB.GetFeatureByUuid(feature.Uuid)

	t.Run("should return error if not authorized", func(t *testing.T) {
		featureUUID := feature.Uuid
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.GetFeatureByUuid)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", featureUUID)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/features/"+featureUUID, nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return feature if user is authorized", func(t *testing.T) {
		featureUUID := feature.Uuid

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.GetFeatureByUuid)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", featureUUID)
		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodGet, "/features/"+featureUUID, nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var returnedFeature db.WorkspaceFeatures
		err = json.Unmarshal(rr.Body.Bytes(), &returnedFeature)
		assert.NoError(t, err)
		assert.Equal(t, feature.Name, returnedFeature.Name)
		assert.Equal(t, feature.Url, returnedFeature.Url)
		assert.Equal(t, feature.Priority, returnedFeature.Priority)
	})
}

func TestCreateOrEditFeaturePhase(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

	person := db.Person{
		Uuid:        "uuid",
		OwnerAlias:  "alias",
		UniqueName:  "unique_name",
		OwnerPubKey: "pubkey",
		PriceToMeet: 0,
		Description: "description",
	}
	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        "workspace_uuid",
		Name:        "workspace_name",
		OwnerPubKey: "person.OwnerPubkey",
		Github:      "gtihub",
		Website:     "website",
		Description: "description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	feature := db.WorkspaceFeatures{
		Uuid:          "feature_uuid",
		WorkspaceUuid: workspace.Uuid,
		Name:          "feature_name",
		Url:           "feature_url",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)

	t.Run("should return 401 error if not authorized", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditFeaturePhase)

		featurePhase := db.FeaturePhase{
			Uuid:        "feature_phase_uuid",
			FeatureUuid: feature.Uuid,
			Name:        "feature_phase_name",
			Priority:    0,
		}

		requestBody, _ := json.Marshal(featurePhase)
		req, err := http.NewRequest(http.MethodPost, "/features/phase", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return 406 error if body is not a valid json", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditFeaturePhase)

		invalidJson := []byte(`{"key": "value"`)

		ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/features/phase", bytes.NewReader(invalidJson))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotAcceptable, rr.Code)
	})

	t.Run("should return 401 error if a Feature UUID that does not exist Is passed to the API body", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditFeaturePhase)

		featurePhase := db.FeaturePhase{
			Uuid:        "feature_phase_uuid",
			FeatureUuid: "non-existent-uuid",
			Name:        "feature_phase_name",
			Priority:    0,
		}

		requestBody, _ := json.Marshal(featurePhase)

		ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/features/phase", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should successfully user can add a feature phase when the right conditions are met", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditFeaturePhase)

		featurePhase := db.FeaturePhase{
			Uuid:        "feature_phase_uuid",
			FeatureUuid: feature.Uuid,
			Name:        "feature_phase_name",
			Priority:    0,
		}

		requestBody, _ := json.Marshal(featurePhase)

		ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/features/phase", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		createdFeaturePhase, _ := db.TestDB.GetFeaturePhaseByUuid(feature.Uuid, featurePhase.Uuid)

		assert.Equal(t, featurePhase.Name, createdFeaturePhase.Name)
		assert.Equal(t, featurePhase.FeatureUuid, createdFeaturePhase.FeatureUuid)
		assert.Equal(t, featurePhase.Priority, createdFeaturePhase.Priority)
	})

	t.Run("should successfully create a feature phase with all new fields", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditFeaturePhase)

		featurePhase := db.FeaturePhase{
			Uuid:         "feature_phase_uuid_full",
			FeatureUuid:  feature.Uuid,
			Name:         "feature_phase_name",
			Priority:     0,
			PhasePurpose: "Test phase purpose",
			PhaseOutcome: "Expected test outcome",
			PhaseScope:   "Test phase scope",
		}

		requestBody, _ := json.Marshal(featurePhase)

		ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/features/phase", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		createdFeaturePhase, _ := db.TestDB.GetFeaturePhaseByUuid(feature.Uuid, featurePhase.Uuid)

		assert.Equal(t, featurePhase.Name, createdFeaturePhase.Name)
		assert.Equal(t, featurePhase.FeatureUuid, createdFeaturePhase.FeatureUuid)
		assert.Equal(t, featurePhase.Priority, createdFeaturePhase.Priority)
		assert.Equal(t, featurePhase.PhasePurpose, createdFeaturePhase.PhasePurpose)
		assert.Equal(t, featurePhase.PhaseOutcome, createdFeaturePhase.PhaseOutcome)
		assert.Equal(t, featurePhase.PhaseScope, createdFeaturePhase.PhaseScope)
	})

	t.Run("should successfully create a feature phase with partial new fields", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditFeaturePhase)

		featurePhase := db.FeaturePhase{
			Uuid:         "feature_phase_uuid_partial",
			FeatureUuid:  feature.Uuid,
			Name:         "feature_phase_name",
			Priority:     0,
			PhasePurpose: "Test phase purpose",
			PhaseScope:   "Test phase scope",
		}

		requestBody, _ := json.Marshal(featurePhase)

		ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/features/phase", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		createdFeaturePhase, _ := db.TestDB.GetFeaturePhaseByUuid(feature.Uuid, featurePhase.Uuid)

		assert.Equal(t, featurePhase.Name, createdFeaturePhase.Name)
		assert.Equal(t, featurePhase.PhasePurpose, createdFeaturePhase.PhasePurpose)
		assert.Equal(t, "", createdFeaturePhase.PhaseOutcome)
		assert.Equal(t, featurePhase.PhaseScope, createdFeaturePhase.PhaseScope)
	})
}

func TestGetFeaturePhases(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	oHandler := NewFeatureHandler(db.TestDB)

	person := db.Person{
		Uuid:        "uuid",
		OwnerAlias:  "alias",
		UniqueName:  "unique_name",
		OwnerPubKey: "pubkey",
		PriceToMeet: 0,
		Description: "description",
	}
	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        "workspace_uuid",
		Name:        "workspace_name",
		OwnerPubKey: "person.OwnerPubkey",
		Github:      "gtihub",
		Website:     "website",
		Description: "description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	feature := db.WorkspaceFeatures{
		Uuid:          "feature_uuid",
		WorkspaceUuid: workspace.Uuid,
		Name:          "feature_name",
		Url:           "feature_url",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)

	featurePhase := db.FeaturePhase{
		Uuid:        "feature_phase_uuid",
		FeatureUuid: feature.Uuid,
		Name:        "feature_phase_name",
		Priority:    0,
	}
	db.TestDB.CreateOrEditFeaturePhase(featurePhase)

	ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)

	t.Run("Should test that it throws a 401 error if a user is not authorized", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/features/"+feature.Uuid+"/phase", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(oHandler.GetFeaturePhases).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Should test that the workspace features phases array returned from the API has the feature phases created", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodGet, "/features/"+feature.Uuid+"/phase", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(oHandler.GetFeaturePhases).ServeHTTP(rr, req)

		var returnedFeaturePhases []db.FeaturePhase
		err = json.Unmarshal(rr.Body.Bytes(), &returnedFeaturePhases)
		assert.NoError(t, err)

		updatedFeaturePhases := db.TestDB.GetPhasesByFeatureUuid(feature.Uuid)

		for i := range updatedFeaturePhases {
			created := updatedFeaturePhases[i].Created.In(time.UTC)
			updated := updatedFeaturePhases[i].Updated.In(time.UTC)
			updatedFeaturePhases[i].Created = &created
			updatedFeaturePhases[i].Updated = &updated
		}

		assert.Equal(t, returnedFeaturePhases, updatedFeaturePhases)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestGetFeaturePhaseByUUID(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

	person := db.Person{
		Uuid:        "uuid",
		OwnerAlias:  "alias",
		UniqueName:  "unique_name",
		OwnerPubKey: "pubkey",
		PriceToMeet: 0,
		Description: "description",
	}
	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        "workspace_uuid",
		Name:        "workspace_name",
		OwnerPubKey: "person.OwnerPubkey",
		Github:      "gtihub",
		Website:     "website",
		Description: "description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	feature := db.WorkspaceFeatures{
		Uuid:          "feature_uuid",
		WorkspaceUuid: workspace.Uuid,
		Name:          "feature_name",
		Url:           "feature_url",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)

	featurePhase := db.FeaturePhase{
		Uuid:        "feature_phase_uuid",
		FeatureUuid: feature.Uuid,
		Name:        "feature_phase_name",
		Priority:    0,
	}
	db.TestDB.CreateOrEditFeaturePhase(featurePhase)

	ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)

	t.Run("Should test that it throws a 401 error if a user is not authorized", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, feature.Uuid+"/phase/"+featurePhase.Uuid, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetFeaturePhaseByUUID).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Should test that the workspace features phases returned from the API has the feature phases created", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodGet, feature.Uuid+"/phase/"+featurePhase.Uuid, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetFeaturePhaseByUUID).ServeHTTP(rr, req)

		var returnedFeaturePhases db.FeaturePhase
		err = json.Unmarshal(rr.Body.Bytes(), &returnedFeaturePhases)
		assert.NoError(t, err)

		updatedFeaturePhase, err := db.TestDB.GetFeaturePhaseByUuid(feature.Uuid, featurePhase.Uuid)
		if err != nil {
			t.Fatal(err)
		}

		updatedFeaturePhase.Created = returnedFeaturePhases.Created
		updatedFeaturePhase.Updated = returnedFeaturePhases.Updated

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, updatedFeaturePhase, returnedFeaturePhases)
	})

	t.Run("Should return feature phase with all new fields", func(t *testing.T) {

		fullFeaturePhase := db.FeaturePhase{
			Uuid:         "feature_phase_uuid_full_get",
			FeatureUuid:  feature.Uuid,
			Name:         "feature_phase_name",
			Priority:     0,
			PhasePurpose: "Test phase purpose",
			PhaseOutcome: "Expected test outcome",
			PhaseScope:   "Test phase scope",
		}
		db.TestDB.CreateOrEditFeaturePhase(fullFeaturePhase)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("phase_uuid", fullFeaturePhase.Uuid)
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			feature.Uuid+"/phase/"+fullFeaturePhase.Uuid,
			nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetFeaturePhaseByUUID).ServeHTTP(rr, req)

		var returnedFeaturePhase db.FeaturePhase
		err = json.Unmarshal(rr.Body.Bytes(), &returnedFeaturePhase)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, fullFeaturePhase.PhasePurpose, returnedFeaturePhase.PhasePurpose)
		assert.Equal(t, fullFeaturePhase.PhaseOutcome, returnedFeaturePhase.PhaseOutcome)
		assert.Equal(t, fullFeaturePhase.PhaseScope, returnedFeaturePhase.PhaseScope)
	})

	t.Run("Should handle empty optional fields correctly", func(t *testing.T) {

		minimalFeaturePhase := db.FeaturePhase{
			Uuid:        "feature_phase_uuid_minimal_get",
			FeatureUuid: feature.Uuid,
			Name:        "feature_phase_name",
			Priority:    0,
		}
		db.TestDB.CreateOrEditFeaturePhase(minimalFeaturePhase)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("phase_uuid", minimalFeaturePhase.Uuid)
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			feature.Uuid+"/phase/"+minimalFeaturePhase.Uuid,
			nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetFeaturePhaseByUUID).ServeHTTP(rr, req)

		var returnedFeaturePhase db.FeaturePhase
		err = json.Unmarshal(rr.Body.Bytes(), &returnedFeaturePhase)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, returnedFeaturePhase.PhasePurpose)
		assert.Empty(t, returnedFeaturePhase.PhaseOutcome)
		assert.Empty(t, returnedFeaturePhase.PhaseScope)
	})
}

func TestDeleteFeaturePhase(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

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
	workspace = db.TestDB.GetWorkspaceByUuid(workspace.Uuid)

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
		Name:        "test-feature-phase",
		Priority:    0,
	}
	db.TestDB.CreateOrEditFeaturePhase(featurePhase)

	ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)

	t.Run("should return error if not authorized", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.DeleteFeaturePhase)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodDelete, "/features/"+feature.Uuid+"/phase/"+featurePhase.Uuid, nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should delete feature phase on successful delete", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.DeleteFeaturePhase)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodDelete, "/features/"+feature.Uuid+"/phase/"+featurePhase.Uuid, nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		deletedFeaturePhase, err := db.TestDB.GetFeaturePhaseByUuid(feature.Uuid, featurePhase.Uuid)
		assert.Error(t, err)
		assert.Equal(t, "no phase found", err.Error())
		assert.Equal(t, db.FeaturePhase{}, deletedFeaturePhase)
	})
}

func TestCreateOrEditStory(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

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
	workspace = db.TestDB.GetWorkspaceByUuid(workspace.Uuid)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)

	featureStory := db.FeatureStory{
		Uuid:        uuid.New().String(),
		FeatureUuid: feature.Uuid,
		Description: "test-description",
		Priority:    0,
	}

	t.Run("should return 401 error if not authorized", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditStory)

		requestBody, _ := json.Marshal(featureStory)
		req, err := http.NewRequest(http.MethodPost, "/features/story", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return 406 error if body is not a valid json", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditStory)

		invalidJson := []byte(`{"key": "value"`)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/features/story", bytes.NewReader(invalidJson))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotAcceptable, rr.Code)
	})

	t.Run("should successfully add feature story if request is valid", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.CreateOrEditStory)

		requestBody, _ := json.Marshal(featureStory)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/features/story", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		createdStory, err := db.TestDB.GetFeatureStoryByUuid(featureStory.FeatureUuid, featureStory.Uuid)
		assert.NoError(t, err)
		assert.Equal(t, featureStory.Description, createdStory.Description)
		assert.Equal(t, featureStory.Priority, createdStory.Priority)
		assert.Equal(t, featureStory.FeatureUuid, createdStory.FeatureUuid)
	})
}

func TestGetStoriesByFeatureUuid(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	oHandler := NewFeatureHandler(db.TestDB)

	person := db.Person{
		Uuid:        "uuid",
		OwnerAlias:  "alias",
		UniqueName:  "unique_name",
		OwnerPubKey: "pubkey",
		PriceToMeet: 0,
		Description: "description",
	}
	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        "workspace_uuid",
		Name:        "workspace_name",
		OwnerPubKey: "person.OwnerPubkey",
		Github:      "gtihub",
		Website:     "website",
		Description: "description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)

	feature := db.WorkspaceFeatures{
		Uuid:          "feature_uuid",
		WorkspaceUuid: workspace.Uuid,
		Name:          "feature_name",
		Url:           "feature_url",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)

	newStory := db.FeatureStory{
		Uuid:        "feature_story_uuid",
		FeatureUuid: feature.Uuid,
		Description: "feature_story_description",
		Priority:    0,
	}
	db.TestDB.CreateOrEditFeatureStory(newStory)

	ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)

	t.Run("Should test that it throws a 401 error if a user is not authorized", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/"+feature.Uuid+"/story", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(oHandler.GetStoriesByFeatureUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Should test that the workspace features stories array returned from the API has the feature stories created", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodGet, "/"+feature.Uuid+"/story", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(oHandler.GetStoriesByFeatureUuid).ServeHTTP(rr, req)

		var returnedFeatureStory []db.FeatureStory
		err = json.Unmarshal(rr.Body.Bytes(), &returnedFeatureStory)
		assert.NoError(t, err)

		updatedFeatureStory, err := db.TestDB.GetFeatureStoriesByFeatureUuid(feature.Uuid)
		if err != nil {
			t.Fatal(err)
		}

		for i := range updatedFeatureStory {
			created := updatedFeatureStory[i].Created.In(time.UTC)
			updated := updatedFeatureStory[i].Updated.In(time.UTC)
			updatedFeatureStory[i].Created = &created
			updatedFeatureStory[i].Updated = &updated
		}

		assert.Equal(t, returnedFeatureStory, updatedFeatureStory)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestGetStoryByUuid(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

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
	workspace = db.TestDB.GetWorkspaceByUuid(workspace.Uuid)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)

	featureStory := db.FeatureStory{
		Uuid:        uuid.New().String(),
		FeatureUuid: feature.Uuid,
		Description: "test-description",
		Priority:    0,
	}
	db.TestDB.CreateOrEditFeatureStory(featureStory)

	t.Run("should return 401 error if not authorized", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.GetStoryByUuid)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("story_uuid", featureStory.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/features/"+feature.Uuid+"/story/"+featureStory.Uuid, nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return feature story if user is authorized", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.GetStoryByUuid)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("story_uuid", featureStory.Uuid)
		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodGet, "/features/"+feature.Uuid+"/story/"+featureStory.Uuid, nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var returnedStory db.FeatureStory
		err = json.Unmarshal(rr.Body.Bytes(), &returnedStory)
		assert.NoError(t, err)
		assert.Equal(t, featureStory.Description, returnedStory.Description)
		assert.Equal(t, featureStory.Priority, returnedStory.Priority)
		assert.Equal(t, featureStory.FeatureUuid, returnedStory.FeatureUuid)
	})
}

func TestDeleteStory(t *testing.T) {

	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

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
	workspace = db.TestDB.GetWorkspaceByUuid(workspace.Uuid)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
	}
	db.TestDB.CreateOrEditFeature(feature)

	featureStory := db.FeatureStory{
		Uuid:        uuid.New().String(),
		FeatureUuid: feature.Uuid,
		Description: "test-description",
		Priority:    0,
	}
	db.TestDB.CreateOrEditFeatureStory(featureStory)

	t.Run("should return 401 error if user not authorized", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.DeleteStory)

		req, err := http.NewRequest(http.MethodDelete, "/"+feature.Uuid+"/story/"+featureStory.Uuid, nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should successfully delete feature story if request is valid", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.DeleteStory)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("story_uuid", featureStory.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodDelete, "/"+feature.Uuid+"/story/"+featureStory.Uuid, nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		deletedFeatureStory, _ := db.TestDB.GetFeatureStoryByUuid(feature.Uuid, featureStory.Uuid)
		assert.Equal(t, db.FeatureStory{}, deletedFeatureStory)

	})

}

func TestGetBountiesByFeatureAndPhaseUuid(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

	db.CleanTestData()

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
	workspace = db.TestDB.GetWorkspaceByUuid(workspace.Uuid)

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
		Name:        "test-feature-phase",
		Priority:    0,
	}
	db.TestDB.CreateOrEditFeaturePhase(featurePhase)

	bounty := db.NewBounty{
		OwnerID:       person.OwnerPubKey,
		WorkspaceUuid: workspace.Uuid,
		Title:         "test-bounty",
		PhaseUuid:     featurePhase.Uuid,
		Description:   "test-description",
		Price:         1000,
		Type:          "coding_task",
		Assignee:      "",
	}
	db.TestDB.CreateOrEditBounty(bounty)

	ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)

	t.Run("should return 401 error if not authorized", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/features/"+feature.Uuid+"/phase/"+featurePhase.Uuid+"/bounty", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return the correct bounty if user is authorized", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodGet, "/features/"+feature.Uuid+"/phase/"+featurePhase.Uuid+"/bounty", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		var returnedBounties []db.BountyResponse
		err = json.Unmarshal(rr.Body.Bytes(), &returnedBounties)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 1, len(returnedBounties))
		assert.Equal(t, bounty.Title, returnedBounties[0].Bounty.Title)
		assert.Equal(t, bounty.Description, returnedBounties[0].Bounty.Description)
		assert.Equal(t, bounty.Price, returnedBounties[0].Bounty.Price)
		assert.Equal(t, bounty.PhaseUuid, returnedBounties[0].Bounty.PhaseUuid)
	})

	t.Run("should phase return the correct bounty response structure", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodGet, "/features/"+feature.Uuid+"/phase/"+featurePhase.Uuid+"/bounty", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		var returnedBounties []db.BountyResponse
		err = json.Unmarshal(rr.Body.Bytes(), &returnedBounties)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, 1, len(returnedBounties))

		expectedBounty := db.BountyResponse{
			Bounty: db.NewBounty{
				ID:                      returnedBounties[0].Bounty.ID,
				OwnerID:                 person.OwnerPubKey,
				Paid:                    false,
				Show:                    false,
				Completed:               false,
				Type:                    "coding_task",
				Award:                   "",
				AssignedHours:           0,
				CommitmentFee:           0,
				Price:                   1000,
				Title:                   "test-bounty",
				Tribe:                   "",
				Assignee:                "",
				TicketUrl:               "",
				OrgUuid:                 workspace.Uuid,
				WorkspaceUuid:           workspace.Uuid,
				Description:             "test-description",
				WantedType:              "",
				Deliverables:            "",
				GithubDescription:       false,
				OneSentenceSummary:      "",
				EstimatedSessionLength:  "",
				EstimatedCompletionDate: "",
				Created:                 0,
				Updated:                 nil,
				PhaseUuid:               featurePhase.Uuid,
				PhasePriority:           0,
				PaymentPending:          false,
				PaymentFailed:           false,
			},
			Assignee: db.Person{},
			Owner: db.Person{
				ID:          returnedBounties[0].Owner.ID,
				Uuid:        person.Uuid,
				OwnerPubKey: person.OwnerPubKey,
				OwnerAlias:  person.OwnerAlias,
				UniqueName:  person.UniqueName,
				Description: person.Description,
				Tags:        pq.StringArray{},
				Img:         "",
			},
			Organization: db.WorkspaceShort{
				Uuid: workspace.Uuid,
				Name: workspace.Name,
				Img:  "",
			},
			Workspace: db.WorkspaceShort{
				Uuid: workspace.Uuid,
				Name: workspace.Name,
				Img:  "",
			},
		}

		assert.Equal(t, expectedBounty, returnedBounties[0])
	})

	t.Run("should return 404 if feature or phase UUID is invalid", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", "invalid-feature-uuid")
		rctx.URLParams.Add("phase_uuid", "invalid-phase-uuid")
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodGet, "/features/invalid-feature-uuid/phase/invalid-phase-uuid/bounty", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("should handle pagination correctly", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			fmt.Sprintf("/features/%s/phase/%s/bounty?limit=1&offset=0", feature.Uuid, featurePhase.Uuid),
			nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("should handle search parameter", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			fmt.Sprintf("/features/%s/phase/%s/bounty?search=test", feature.Uuid, featurePhase.Uuid),
			nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("should handle status filters", func(t *testing.T) {
		testCases := []struct {
			name   string
			query  string
			status string
		}{
			{"open bounties", "Open=true", ""},
			{"assigned bounties", "Assigned=true", person.OwnerPubKey},
			{"completed bounties", "Completed=true", person.OwnerPubKey},
			{"paid bounties", "Paid=true", person.OwnerPubKey},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {

				statusBounty := bounty
				statusBounty.Assignee = tc.status
				if tc.name == "completed bounties" {
					statusBounty.Completed = true
				}
				if tc.name == "paid bounties" {
					statusBounty.Paid = true
				}
				db.TestDB.CreateOrEditBounty(statusBounty)

				rctx := chi.NewRouteContext()
				rctx.URLParams.Add("feature_uuid", feature.Uuid)
				rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
				req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
					http.MethodGet,
					fmt.Sprintf("/features/%s/phase/%s/bounty?%s", feature.Uuid, featurePhase.Uuid, tc.query),
					nil)
				assert.NoError(t, err)

				rr := httptest.NewRecorder()
				http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

				assert.Equal(t, http.StatusOK, rr.Code)
			})
		}
	})

	t.Run("should handle language filters", func(t *testing.T) {

		langBounty := bounty
		langBounty.CodingLanguages = pq.StringArray{"golang", "javascript"}
		db.TestDB.CreateOrEditBounty(langBounty)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			fmt.Sprintf("/features/%s/phase/%s/bounty?languages=golang", feature.Uuid, featurePhase.Uuid),
			nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("should handle tags filter", func(t *testing.T) {

		taggedBounty := db.NewBounty{
			OwnerID:       person.OwnerPubKey,
			WorkspaceUuid: workspace.Uuid,
			Title:         "tagged-test-bounty",
			PhaseUuid:     featurePhase.Uuid,
			Description:   "test-description-with-tags",
			Price:         1000,
			Type:          "coding_task",
			Assignee:      "",
		}
		db.TestDB.CreateOrEditBounty(taggedBounty)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			fmt.Sprintf("/features/%s/phase/%s/bounty?tags=urgent", feature.Uuid, featurePhase.Uuid),
			nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("should handle sorting parameters", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			fmt.Sprintf("/features/%s/phase/%s/bounty?sortBy=price&direction=DESC", feature.Uuid, featurePhase.Uuid),
			nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("should handle multiple filters together", func(t *testing.T) {

		multiBounty := db.NewBounty{
			ID:              1,
			OwnerID:         person.OwnerPubKey,
			WorkspaceUuid:   workspace.Uuid,
			Title:           "multi-filter-test-bounty",
			PhaseUuid:       featurePhase.Uuid,
			FeatureUuid:     feature.Uuid,
			Description:     "test-description-multi-filter",
			Price:           1000,
			Type:            "coding_task",
			Assignee:        "",
			CodingLanguages: pq.StringArray{"golang", "javascript"},
			Paid:            false,
			Completed:       false,
		}
		_, err := db.TestDB.CreateOrEditBounty(multiBounty)
		assert.NoError(t, err)

		bounties, err := db.TestDB.GetBountiesByFeatureAndPhaseUuid(feature.Uuid, featurePhase.Uuid, &http.Request{URL: &url.URL{RawQuery: ""}})
		assert.NoError(t, err)
		assert.NotEmpty(t, bounties)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)

		reqURL := fmt.Sprintf("/features/%s/phase/%s/bounty?Open=true&languages=golang&sortBy=price&direction=DESC",
			feature.Uuid, featurePhase.Uuid)
		req, err := http.NewRequestWithContext(
			context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			reqURL,
			nil,
		)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var returnedBounties []db.BountyResponse
		err = json.Unmarshal(rr.Body.Bytes(), &returnedBounties)
		assert.NoError(t, err)
		assert.NotEmpty(t, returnedBounties)
		assert.Equal(t, multiBounty.Title, returnedBounties[0].Bounty.Title)
	})

	t.Run("Valid Feature and Phase UUIDs with No Bounties", func(t *testing.T) {
		emptyFeature := db.WorkspaceFeatures{
			Uuid:          uuid.New().String(),
			WorkspaceUuid: workspace.Uuid,
			Name:          "empty-feature",
		}
		db.TestDB.CreateOrEditFeature(emptyFeature)

		emptyPhase := db.FeaturePhase{
			Uuid:        uuid.New().String(),
			FeatureUuid: emptyFeature.Uuid,
			Name:        "empty-phase",
		}
		db.TestDB.CreateOrEditFeaturePhase(emptyPhase)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", emptyFeature.Uuid)
		rctx.URLParams.Add("phase_uuid", emptyPhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			fmt.Sprintf("/features/%s/phase/%s/bounty", emptyFeature.Uuid, emptyPhase.Uuid),
			nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("Empty Feature UUID", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", "")
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			fmt.Sprintf("/features/%s/phase/%s/bounty", "", featurePhase.Uuid),
			nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("Empty Phase UUID", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", "")
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			fmt.Sprintf("/features/%s/phase/%s/bounty", feature.Uuid, ""),
			nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("Invalid UUID Format", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", "invalid-uuid-format")
		rctx.URLParams.Add("phase_uuid", "invalid-uuid-format")
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			"/features/invalid-uuid-format/phase/invalid-uuid-format/bounty",
			nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("Feature and Phase UUIDs with Special Characters", func(t *testing.T) {
		specialFeatureUUID := "test!@#$%^&*()"
		specialPhaseUUID := "phase!@#$%^&*()"

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", specialFeatureUUID)
		rctx.URLParams.Add("phase_uuid", specialPhaseUUID)

		encodedFeatureUUID := url.QueryEscape(specialFeatureUUID)
		encodedPhaseUUID := url.QueryEscape(specialPhaseUUID)

		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			fmt.Sprintf("/features/%s/phase/%s/bounty", encodedFeatureUUID, encodedPhaseUUID),
			nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("Null Feature and Phase UUIDs", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodGet,
			"/features/null/phase/null/bounty",
			nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestGetBountiesCountByFeatureAndPhaseUuid(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

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
	workspace = db.TestDB.GetWorkspaceByUuid(workspace.Uuid)

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
		Name:        "test-feature-phase",
		Priority:    0,
	}
	db.TestDB.CreateOrEditFeaturePhase(featurePhase)

	bounty := db.NewBounty{
		OwnerID:       person.OwnerPubKey,
		WorkspaceUuid: workspace.Uuid,
		Title:         "test-bounty",
		PhaseUuid:     featurePhase.Uuid,
		Description:   "test-description",
		Price:         1000,
		Type:          "coding_task",
		Assignee:      "",
	}
	db.TestDB.CreateOrEditBounty(bounty)

	ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)

	t.Run("should return 401 error if not authorized", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx), http.MethodGet, "/features/"+feature.Uuid+"/phase/"+featurePhase.Uuid+"/bounty/count", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesCountByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return the correct bounty count if user is authorized", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("feature_uuid", feature.Uuid)
		rctx.URLParams.Add("phase_uuid", featurePhase.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodGet, "/features/"+feature.Uuid+"/phase/"+featurePhase.Uuid+"/bounty/count", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetBountiesCountByFeatureAndPhaseUuid).ServeHTTP(rr, req)

		var returnedBountiesCount int64
		err = json.Unmarshal(rr.Body.Bytes(), &returnedBountiesCount)
		assert.NoError(t, err)

		bountiesCount := db.TestDB.GetBountiesCountByFeatureAndPhaseUuid(feature.Uuid, featurePhase.Uuid, req)

		assert.Equal(t, returnedBountiesCount, bountiesCount)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestGetFeatureStories(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

	person := db.Person{
		Uuid:        uuid.New().String(),
		OwnerAlias:  "test-get-feature-stories-alias",
		UniqueName:  "test-get-feature-stories-unique-name",
		OwnerPubKey: "test-get-feature-stories-pubkey",
		PriceToMeet: 0,
		Description: "test-get-feature-stories-description",
	}
	db.TestDB.CreateOrEditPerson(person)

	workspace := db.Workspace{
		Uuid:        uuid.New().String(),
		Name:        "test-get-feature-stories-workspace-name",
		OwnerPubKey: person.OwnerPubKey,
		Github:      "https://github.com/test",
		Website:     "https://www.testwebsite.com",
		Description: "test-get-feature-stories-description",
	}
	db.TestDB.CreateOrEditWorkspace(workspace)
	workspace = db.TestDB.GetWorkspaceByUuid(workspace.Uuid)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-get-feature-stories-feature-name",
		Url:           "https://github.com/test-get-feature-stories-feature-url",
		Priority:      0,
	}

	feature2 := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-get-feature-stories-feature-name-2",
		Url:           "https://github.com/test-get-feature-stories-feature-url-2",
		Priority:      0,
	}

	db.TestDB.CreateOrEditFeature(feature)
	db.TestDB.CreateOrEditFeature(feature2)

	ctx := context.WithValue(context.Background(), auth.ContextKey, workspace.OwnerPubKey)

	story := db.FeatureStories{
		UserStory: "This is a test user story",
		Rationale: "This is a test rationale",
		Order:     1,
	}

	story2 := db.FeatureStories{
		UserStory: "This is a test user story 2",
		Rationale: "This is a test rationale 2",
		Order:     2,
	}

	story3 := db.FeatureStories{
		UserStory: "This is a test user story 3",
		Rationale: "This is a test rationale 3",
		Order:     3,
	}

	story4 := db.FeatureStories{
		UserStory: "This is a test user story 4",
		Rationale: "This is a test rationale 4",
		Order:     4,
	}

	story5 := db.FeatureStories{
		UserStory: "This is a test user story 5",
		Rationale: "This is a test rationale 5",
		Order:     5,
	}

	story6 := db.FeatureStories{
		UserStory: "This is a test user story 6",
		Rationale: "This is a test rationale 6",
		Order:     6,
	}

	stories := []db.FeatureStories{
		story,
		story2,
		story3,
	}

	stories2 := []db.FeatureStories{
		story4,
		story5,
		story6,
	}

	featureStories := db.FeatureStoriesReponse{
		Output: db.FeatureOutput{
			FeatureUuid:    feature.Uuid,
			FeatureContext: "Feature Context",
			Stories:        stories,
		},
	}

	featureStories2 := db.FeatureStoriesReponse{
		Output: db.FeatureOutput{
			FeatureUuid:    "Fake-feature-uuid",
			FeatureContext: "Feature Context",
			Stories:        stories2,
		},
	}

	requestBody, _ := json.Marshal(featureStories)
	requestBody2, _ := json.Marshal(featureStories2)

	t.Run("Should add user stories from stakwork to the feature stories table", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodPost, "/features/stories", bytes.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetFeatureStories).ServeHTTP(rr, req)

		var featureStoriesReponse string
		err = json.Unmarshal(rr.Body.Bytes(), &featureStoriesReponse)
		assert.NoError(t, err)

		featureStories, _ := db.TestDB.GetFeatureStoriesByFeatureUuid(feature.Uuid)
		featureStoriesCount := len(featureStories)

		assert.Equal(t, int64(featureStoriesCount), int64(3))
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Should not add user stories from stakwork to the feature stories table if the feature uuid is not found", func(t *testing.T) {
		rctx := chi.NewRouteContext()
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx), http.MethodPost, "/features/stories", bytes.NewReader(requestBody2))
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		http.HandlerFunc(fHandler.GetFeatureStories).ServeHTTP(rr, req)

		var featureStoriesReponse string
		err = json.Unmarshal(rr.Body.Bytes(), &featureStoriesReponse)
		assert.NoError(t, err)

		featureStories, _ := db.TestDB.GetFeatureStoriesByFeatureUuid(feature2.Uuid)
		featureStoriesCount := len(featureStories)

		assert.Equal(t, int64(featureStoriesCount), int64(0))
		assert.Equal(t, http.StatusNotAcceptable, rr.Code)
	})
}

func TestGetFeaturesByWorkspaceUuid(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	oHandler := NewWorkspaceHandler(db.TestDB)

	db.CleanTestData()

	tests := []struct {
		name           string
		pubKeyFromAuth string
		workspaceUUID  string
		setupMocks     func() (string, interface{})
		expectedStatus int
	}{
		{
			name:           "should return error if a user is not authorized",
			pubKeyFromAuth: "",
			workspaceUUID:  "valid-uuid",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Missing UUID Parameter",
			pubKeyFromAuth: "validPubKey",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Special Characters in UUID",
			pubKeyFromAuth: "validPubKey",
			workspaceUUID:  "1234-5678-!@#$",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Non-Existent UUID",
			pubKeyFromAuth: "validPubKey",
			workspaceUUID:  "nonexistentuuid",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "created feature should be present in the returned array",
			pubKeyFromAuth: "validPubKey",
			workspaceUUID:  "valid-uuid",
			setupMocks: func() (string, interface{}) {
				person := db.Person{
					Uuid:        uuid.New().String(),
					OwnerAlias:  "alias",
					UniqueName:  "unique_name",
					OwnerPubKey: "validPubKey",
					PriceToMeet: 0,
					Description: "description",
				}
				db.TestDB.CreateOrEditPerson(person)

				workspace := db.Workspace{
					Uuid:        uuid.New().String(),
					Name:        "unique_workspace_name",
					OwnerPubKey: person.OwnerPubKey,
					Github:      "github",
					Website:     "website",
					Description: "description",
				}
				db.TestDB.CreateOrEditWorkspace(workspace)

				feature := db.WorkspaceFeatures{
					Uuid:          "mock-feature-uuid1",
					WorkspaceUuid: workspace.Uuid,
					Name:          "feature_name",
					Url:           "https://www.bountieswebsite.com",
					Priority:      0,
				}
				db.TestDB.CreateOrEditFeature(feature)

				return workspace.Uuid, []db.WorkspaceFeatures{
					{
						Uuid:     "mock-feature-uuid1",
						Name:     "feature_name",
						Url:      "https://www.bountieswebsite.com",
						Priority: 0,
					},
				}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Large Number of Features",
			pubKeyFromAuth: "validPubKey",
			workspaceUUID:  "valid-uuid",
			setupMocks: func() (string, interface{}) {
				person := db.Person{
					Uuid:        uuid.New().String(),
					OwnerAlias:  "alias",
					UniqueName:  "unique_name",
					OwnerPubKey: "validPubKey",
					PriceToMeet: 0,
					Description: "description",
				}
				db.TestDB.CreateOrEditPerson(person)

				workspace := db.Workspace{
					Uuid:        uuid.New().String(),
					Name:        "unique_workspace_name",
					OwnerPubKey: person.OwnerPubKey,
					Github:      "github",
					Website:     "website",
					Description: "description",
				}
				db.TestDB.CreateOrEditWorkspace(workspace)

				var features []db.WorkspaceFeatures
				for i := 0; i < 1000; i++ {
					feature := db.WorkspaceFeatures{
						Uuid:          uuid.New().String(),
						WorkspaceUuid: workspace.Uuid,
						Name:          fmt.Sprintf("feature_%d", i),
						Url:           fmt.Sprintf("https://example.com/feature_%d", i),
						Priority:      i,
					}
					db.TestDB.CreateOrEditFeature(feature)
					features = append(features, feature)
				}

				return workspace.Uuid, features
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Empty Workspace with Status Filter",
			pubKeyFromAuth: "validPubKey",
			workspaceUUID:  "valid-uuid",
			setupMocks: func() (string, interface{}) {
				person := db.Person{
					Uuid:        uuid.New().String(),
					OwnerPubKey: "validPubKey",
				}
				db.TestDB.CreateOrEditPerson(person)

				workspace := db.Workspace{
					Uuid:        uuid.New().String(),
					OwnerPubKey: person.OwnerPubKey,
				}
				db.TestDB.CreateOrEditWorkspace(workspace)

				return workspace.Uuid, []db.WorkspaceFeatures{}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Features with Different Statuses",
			pubKeyFromAuth: "validPubKey",
			workspaceUUID:  "valid-uuid",
			setupMocks: func() (string, interface{}) {
				person := db.Person{
					Uuid:        uuid.New().String(),
					OwnerPubKey: "validPubKey",
				}
				db.TestDB.CreateOrEditPerson(person)

				workspace := db.Workspace{
					Uuid:        uuid.New().String(),
					OwnerPubKey: person.OwnerPubKey,
				}
				db.TestDB.CreateOrEditWorkspace(workspace)

				activeFeature := db.WorkspaceFeatures{
					Uuid:          uuid.New().String(),
					WorkspaceUuid: workspace.Uuid,
					Name:          "Active Feature",
					FeatStatus:    db.ActiveFeature,
				}
				db.TestDB.CreateOrEditFeature(activeFeature)

				archivedFeature := db.WorkspaceFeatures{
					Uuid:          uuid.New().String(),
					WorkspaceUuid: workspace.Uuid,
					Name:          "Archived Feature",
					FeatStatus:    db.ArchivedFeature,
				}
				db.TestDB.CreateOrEditFeature(archivedFeature)

				return workspace.Uuid, []db.WorkspaceFeatures{activeFeature}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Features with Custom Sorting",
			pubKeyFromAuth: "validPubKey",
			workspaceUUID:  "valid-uuid",
			setupMocks: func() (string, interface{}) {
				person := db.Person{
					Uuid:        uuid.New().String(),
					OwnerPubKey: "validPubKey",
				}
				db.TestDB.CreateOrEditPerson(person)

				workspace := db.Workspace{
					Uuid:        uuid.New().String(),
					OwnerPubKey: person.OwnerPubKey,
				}
				db.TestDB.CreateOrEditWorkspace(workspace)

				features := []db.WorkspaceFeatures{
					{
						Uuid:          uuid.New().String(),
						WorkspaceUuid: workspace.Uuid,
						Name:          "Feature A",
						Priority:      3,
					},
					{
						Uuid:          uuid.New().String(),
						WorkspaceUuid: workspace.Uuid,
						Name:          "Feature B",
						Priority:      1,
					},
					{
						Uuid:          uuid.New().String(),
						WorkspaceUuid: workspace.Uuid,
						Name:          "Feature C",
						Priority:      2,
					},
				}

				for _, f := range features {
					db.TestDB.CreateOrEditFeature(f)
				}

				return workspace.Uuid, features
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Features with Pagination",
			pubKeyFromAuth: "validPubKey",
			workspaceUUID:  "valid-uuid",
			setupMocks: func() (string, interface{}) {
				person := db.Person{
					Uuid:        uuid.New().String(),
					OwnerPubKey: "validPubKey",
				}
				db.TestDB.CreateOrEditPerson(person)

				workspace := db.Workspace{
					Uuid:        uuid.New().String(),
					OwnerPubKey: person.OwnerPubKey,
				}
				db.TestDB.CreateOrEditWorkspace(workspace)

				var features []db.WorkspaceFeatures
				for i := 0; i < 25; i++ {
					feature := db.WorkspaceFeatures{
						Uuid:          uuid.New().String(),
						WorkspaceUuid: workspace.Uuid,
						Name:          fmt.Sprintf("Feature %d", i),
						Priority:      i,
					}
					db.TestDB.CreateOrEditFeature(feature)
					features = append(features, feature)
				}

				return workspace.Uuid, features
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Features with Special Characters in Names",
			pubKeyFromAuth: "validPubKey",
			workspaceUUID:  "valid-uuid",
			setupMocks: func() (string, interface{}) {
				person := db.Person{
					Uuid:        uuid.New().String(),
					OwnerPubKey: "validPubKey",
				}
				db.TestDB.CreateOrEditPerson(person)

				workspace := db.Workspace{
					Uuid:        uuid.New().String(),
					OwnerPubKey: person.OwnerPubKey,
				}
				db.TestDB.CreateOrEditWorkspace(workspace)

				feature := db.WorkspaceFeatures{
					Uuid:          uuid.New().String(),
					WorkspaceUuid: workspace.Uuid,
					Name:          "Feature !@#$%^&*()",
					Priority:      1,
				}
				db.TestDB.CreateOrEditFeature(feature)

				return workspace.Uuid, []db.WorkspaceFeatures{feature}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Features with Unicode Characters",
			pubKeyFromAuth: "validPubKey",
			workspaceUUID:  "valid-uuid",
			setupMocks: func() (string, interface{}) {
				person := db.Person{
					Uuid:        uuid.New().String(),
					OwnerPubKey: "validPubKey",
				}
				db.TestDB.CreateOrEditPerson(person)

				workspace := db.Workspace{
					Uuid:        uuid.New().String(),
					OwnerPubKey: person.OwnerPubKey,
				}
				db.TestDB.CreateOrEditWorkspace(workspace)

				feature := db.WorkspaceFeatures{
					Uuid:          uuid.New().String(),
					WorkspaceUuid: workspace.Uuid,
					Name:          "测试Feature テスト",
					Priority:      1,
				}
				db.TestDB.CreateOrEditFeature(feature)

				return workspace.Uuid, []db.WorkspaceFeatures{feature}
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var workspaceUUID string
			var expectedBody interface{}

			if tt.setupMocks != nil {
				workspaceUUID, expectedBody = tt.setupMocks()
			}

			req := httptest.NewRequest(http.MethodGet, "/features", nil)
			req = req.WithContext(context.WithValue(req.Context(), auth.ContextKey, tt.pubKeyFromAuth))
			rctx := chi.NewRouteContext()

			if workspaceUUID != "" {
				rctx.URLParams.Add("workspace_uuid", workspaceUUID)
			} else {
				rctx.URLParams.Add("workspace_uuid", tt.workspaceUUID)
			}
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			rec := httptest.NewRecorder()

			oHandler.GetFeaturesByWorkspaceUuid(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if expectedBody != nil {
				var actualBody []db.WorkspaceFeatures
				err := json.NewDecoder(rec.Body).Decode(&actualBody)
				assert.NoError(t, err)

				sort.Slice(expectedBody, func(i, j int) bool {
					return expectedBody.([]db.WorkspaceFeatures)[i].Uuid < expectedBody.([]db.WorkspaceFeatures)[j].Uuid
				})
				sort.Slice(actualBody, func(i, j int) bool {
					return actualBody[i].Uuid < actualBody[j].Uuid
				})

				assert.Len(t, actualBody, len(expectedBody.([]db.WorkspaceFeatures)))

				for i, expectedFeature := range expectedBody.([]db.WorkspaceFeatures) {
					actualFeature := actualBody[i]
					assert.Equal(t, expectedFeature.Uuid, actualFeature.Uuid)
					assert.Equal(t, expectedFeature.Name, actualFeature.Name)
					assert.Equal(t, expectedFeature.Url, actualFeature.Url)
					assert.Equal(t, expectedFeature.Priority, actualFeature.Priority)

					assert.NotNil(t, actualFeature.Created)
					assert.NotNil(t, actualFeature.Updated)
				}
			}
		})
	}
}

func TestUpdateFeatureStatus(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	fHandler := NewFeatureHandler(db.TestDB)

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
	workspace = db.TestDB.GetWorkspaceByUuid(workspace.Uuid)

	feature := db.WorkspaceFeatures{
		Uuid:          uuid.New().String(),
		WorkspaceUuid: workspace.Uuid,
		Name:          "test-feature",
		Url:           "https://github.com/test-feature",
		Priority:      0,
		FeatStatus:    db.ActiveFeature,
	}
	db.TestDB.CreateOrEditFeature(feature)
	feature = db.TestDB.GetFeatureByUuid(feature.Uuid)

	t.Run("should return error if not authorized", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		requestBody := map[string]string{"status": string(db.ArchivedFeature)}
		body, _ := json.Marshal(requestBody)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx),
			http.MethodPut, "/features/"+feature.Uuid+"/status", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return bad request if body is not valid json", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		invalidJson := []byte(`{"status": "archived"`)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodPut, "/features/"+feature.Uuid+"/status", bytes.NewReader(invalidJson))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should return bad request if status is invalid", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		requestBody := map[string]string{"status": "invalid_status"}
		body, _ := json.Marshal(requestBody)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodPut, "/features/"+feature.Uuid+"/status", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should return internal server error if feature not found", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		requestBody := map[string]string{"status": string(db.ArchivedFeature)}
		body, _ := json.Marshal(requestBody)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", "non-existent-uuid")
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodPut, "/features/non-existent-uuid/status", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("should successfully update feature status", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		requestBody := map[string]string{"status": string(db.ArchivedFeature)}
		body, _ := json.Marshal(requestBody)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodPut, "/features/"+feature.Uuid+"/status", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var updatedFeature db.WorkspaceFeatures
		err = json.NewDecoder(rr.Body).Decode(&updatedFeature)
		assert.NoError(t, err)
		assert.Equal(t, db.ArchivedFeature, updatedFeature.FeatStatus)
		assert.Equal(t, feature.Uuid, updatedFeature.Uuid)
	})

	t.Run("Empty UUID Parameter", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		requestBody := map[string]string{"status": string(db.ActiveFeature)}
		body, _ := json.Marshal(requestBody)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", "")
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodPut, "/features//status", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response map[string]string
		err = json.NewDecoder(rr.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "Missing uuid parameter", response["error"])
	})

	t.Run("Boundary UUID Length", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		requestBody := map[string]string{"status": string(db.ActiveFeature)}
		body, _ := json.Marshal(requestBody)

		longUUID := strings.Repeat("a", 100)
		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", longUUID)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodPut, "/features/"+longUUID+"/status", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("Null Request Body", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodPut, "/features/"+feature.Uuid+"/status", nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		var response struct {
			Error string `json:"error"`
		}
		err = json.NewDecoder(rr.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "Request body is required", response.Error)
	})

	t.Run("Empty Request Body", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		emptyBody := map[string]string{}
		body, _ := json.Marshal(emptyBody)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodPut, "/features/"+feature.Uuid+"/status", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("Unauthorized Access with Invalid PubKey", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		requestBody := map[string]string{"status": string(db.ActiveFeature)}
		body, _ := json.Marshal(requestBody)

		ctx := context.WithValue(context.Background(), auth.ContextKey, "invalid-pubkey")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodPut, "/features/"+feature.Uuid+"/status", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		var response struct {
			Error string `json:"error"`
		}
		err = json.NewDecoder(rr.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "Unauthorized: invalid pubkey", response.Error)
	})

	t.Run("Invalid UUID Format", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		requestBody := map[string]string{"status": string(db.ActiveFeature)}
		body, _ := json.Marshal(requestBody)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", "invalid-uuid-format")
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodPut, "/features/invalid-uuid-format/status", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("Valid Request with Active Status", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(fHandler.UpdateFeatureStatus)

		requestBody := map[string]string{"status": string(db.ActiveFeature)}
		body, _ := json.Marshal(requestBody)

		ctx := context.WithValue(context.Background(), auth.ContextKey, person.OwnerPubKey)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("uuid", feature.Uuid)
		req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx),
			http.MethodPut, "/features/"+feature.Uuid+"/status", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var updatedFeature db.WorkspaceFeatures
		err = json.NewDecoder(rr.Body).Decode(&updatedFeature)
		assert.NoError(t, err)
		assert.Equal(t, db.ActiveFeature, updatedFeature.FeatStatus)
		assert.Equal(t, feature.Uuid, updatedFeature.Uuid)
	})
}
