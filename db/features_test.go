package db

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestDeleteFeatureStoryByUuid(t *testing.T) {

	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	person := Person{
		Uuid:        uuid.New().String(),
		OwnerPubKey: "test-pubkey",
		OwnerAlias:  "test-alias",
	}
	TestDB.CreateOrEditPerson(person)

	currentTime := time.Now()

	tests := []struct {
		name        string
		setup       func() (string, string)
		expectError bool
		errorMsg    string
	}{
		{
			name: "Successfully delete existing feature story",
			setup: func() (string, string) {
				featureUuid := uuid.New().String()
				storyUuid := uuid.New().String()

				story := FeatureStory{
					Uuid:        storyUuid,
					FeatureUuid: featureUuid,
					Description: "Test story",
					Priority:    1,
					Created:     &currentTime,
					Updated:     &currentTime,
					CreatedBy:   person.OwnerPubKey,
					UpdatedBy:   person.OwnerPubKey,
				}

				result := TestDB.db.Create(&story)
				assert.NoError(t, result.Error)

				return featureUuid, storyUuid
			},
			expectError: false,
			errorMsg:    "",
		},
		{
			name: "Try to delete non-existent feature story",
			setup: func() (string, string) {
				return uuid.New().String(), uuid.New().String()
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
		{
			name: "Try to delete with empty UUIDs",
			setup: func() (string, string) {
				return "", ""
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
		{
			name: "Try to delete with valid feature UUID but invalid story UUID",
			setup: func() (string, string) {
				featureUuid := uuid.New().String()
				story := FeatureStory{
					Uuid:        uuid.New().String(),
					FeatureUuid: featureUuid,
					Description: "Test story",
					Priority:    1,
					Created:     &currentTime,
					CreatedBy:   person.OwnerPubKey,
				}

				result := TestDB.db.Create(&story)
				assert.NoError(t, result.Error)

				return featureUuid, uuid.New().String()
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
		{
			name: "Try to delete with invalid feature UUID but valid story UUID",
			setup: func() (string, string) {
				featureUuid := uuid.New().String()
				storyUuid := uuid.New().String()

				story := FeatureStory{
					Uuid:        storyUuid,
					FeatureUuid: featureUuid,
					Description: "Test story",
					Priority:    1,
					Created:     &currentTime,
					CreatedBy:   person.OwnerPubKey,
				}

				result := TestDB.db.Create(&story)
				assert.NoError(t, result.Error)

				return uuid.New().String(), storyUuid
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
		{
			name: "Delete story and verify it's gone",
			setup: func() (string, string) {
				featureUuid := uuid.New().String()
				storyUuid := uuid.New().String()

				story := FeatureStory{
					Uuid:        storyUuid,
					FeatureUuid: featureUuid,
					Description: "Test story",
					Priority:    1,
					Created:     &currentTime,
					CreatedBy:   person.OwnerPubKey,
				}

				result := TestDB.db.Create(&story)
				assert.NoError(t, result.Error)

				return featureUuid, storyUuid
			},
			expectError: false,
			errorMsg:    "",
		},
		{
			name: "Try to delete already deleted story",
			setup: func() (string, string) {
				featureUuid := uuid.New().String()
				storyUuid := uuid.New().String()

				story := FeatureStory{
					Uuid:        storyUuid,
					FeatureUuid: featureUuid,
					Description: "Test story",
					Priority:    1,
					Created:     &currentTime,
					CreatedBy:   person.OwnerPubKey,
				}

				TestDB.db.Create(&story)
				TestDB.db.Delete(&story)

				return featureUuid, storyUuid
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
		{
			name: "Case Sensitivity Test",
			setup: func() (string, string) {
				featureUuid := uuid.New().String()
				storyUuid := uuid.New().String()

				story := FeatureStory{
					Uuid:        storyUuid,
					FeatureUuid: featureUuid,
					Description: "Case sensitivity test story",
					Priority:    1,
					Created:     &currentTime,
					CreatedBy:   person.OwnerPubKey,
				}

				result := TestDB.db.Create(&story)
				assert.NoError(t, result.Error)

				return strings.ToUpper(featureUuid), strings.ToUpper(storyUuid)
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
		{
			name: "SQL Injection Attempt",
			setup: func() (string, string) {
				return "' OR '1'='1", "' OR '1'='1"
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
		{
			name: "Invalid UUID Format",
			setup: func() (string, string) {
				return "invalid-uuid-format", "another-invalid-uuid"
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
		{
			name: "Multiple Stories with Same Feature UUID",
			setup: func() (string, string) {
				featureUuid := uuid.New().String()
				storyUuid1 := uuid.New().String()
				storyUuid2 := uuid.New().String()

				story1 := FeatureStory{
					Uuid:        storyUuid1,
					FeatureUuid: featureUuid,
					Description: "First story",
					Priority:    1,
					Created:     &currentTime,
					CreatedBy:   person.OwnerPubKey,
				}

				story2 := FeatureStory{
					Uuid:        storyUuid2,
					FeatureUuid: featureUuid,
					Description: "Second story",
					Priority:    2,
					Created:     &currentTime,
					CreatedBy:   person.OwnerPubKey,
				}

				TestDB.db.Create(&story1)
				TestDB.db.Create(&story2)

				return featureUuid, storyUuid1
			},
			expectError: false,
			errorMsg:    "",
		},
		{
			name: "Special Characters in UUID",
			setup: func() (string, string) {
				return "!@#$%^&*()", "!@#$%^&*()"
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
		{
			name: "Very Long UUID Values",
			setup: func() (string, string) {
				return strings.Repeat("a", 1000), strings.Repeat("b", 1000)
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
		{
			name: "Unicode Characters in UUID",
			setup: func() (string, string) {
				return "测试UUID", "テストUUID"
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
		{
			name: "Concurrent Deletion Attempt",
			setup: func() (string, string) {
				featureUuid := uuid.New().String()
				storyUuid := uuid.New().String()

				story := FeatureStory{
					Uuid:        storyUuid,
					FeatureUuid: featureUuid,
					Description: "Concurrent deletion test",
					Priority:    1,
					Created:     &currentTime,
					CreatedBy:   person.OwnerPubKey,
				}

				TestDB.db.Create(&story)

				go func() {
					TestDB.DeleteFeatureStoryByUuid(featureUuid, storyUuid)
				}()
				time.Sleep(10 * time.Millisecond)

				return featureUuid, storyUuid
			},
			expectError: true,
			errorMsg:    "no story found to delete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			featureUuid, storyUuid := tt.setup()

			err := TestDB.DeleteFeatureStoryByUuid(featureUuid, storyUuid)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, tt.errorMsg, err.Error())
			} else {
				assert.NoError(t, err)

				var story FeatureStory
				result := TestDB.db.Where("feature_uuid = ? AND uuid = ?", featureUuid, storyUuid).First(&story)
				assert.Error(t, result.Error)
				assert.True(t, errors.Is(result.Error, gorm.ErrRecordNotFound))
			}
		})
	}
}

func TestGetBountiesByPhaseUuid(t *testing.T) {
	teardownSuite := SetupSuite(t)
	defer teardownSuite(t)

	currentTime := time.Now().Unix()
	person := Person{
		Uuid:        uuid.New().String(),
		OwnerPubKey: "test-pubkey",
		OwnerAlias:  "test-alias",
	}
	TestDB.CreateOrEditPerson(person)

	tests := []struct {
		name          string
		setup         func() string
		expectedCount int
		validate      func(t *testing.T, bounties []Bounty)
	}{
		{
			name: "Successfully get bounties for existing phase",
			setup: func() string {
				phaseUuid := uuid.New().String()

				bounties := []Bounty{
					{
						ID:          1,
						OwnerID:     person.OwnerPubKey,
						Title:       "Bounty 1",
						Description: "Test bounty 1",
						Price:       1000,
						Type:        "coding_task",
						PhaseUuid:   &phaseUuid,
						Created:     currentTime,
					},
					{
						ID:          2,
						OwnerID:     person.OwnerPubKey,
						Title:       "Bounty 2",
						Description: "Test bounty 2",
						Price:       2000,
						Type:        "coding_task",
						PhaseUuid:   &phaseUuid,
						Created:     currentTime,
					},
				}

				for _, b := range bounties {
					result := TestDB.db.Create(&b)
					assert.NoError(t, result.Error)
				}

				return phaseUuid
			},
			expectedCount: 2,
			validate: func(t *testing.T, bounties []Bounty) {
				assert.Len(t, bounties, 2)
				assert.NotEqual(t, bounties[0].ID, bounties[1].ID)
				assert.Equal(t, *bounties[0].PhaseUuid, *bounties[1].PhaseUuid)
			},
		},
		{
			name: "No bounties for phase",
			setup: func() string {
				return uuid.New().String()
			},
			expectedCount: 0,
			validate: func(t *testing.T, bounties []Bounty) {
				assert.Empty(t, bounties)
			},
		},
		{
			name: "Empty phase UUID",
			setup: func() string {
				return ""
			},
			expectedCount: 0,
			validate: func(t *testing.T, bounties []Bounty) {
				assert.Empty(t, bounties)
			},
		},
		{
			name: "Phase with multiple status bounties",
			setup: func() string {
				phaseUuid := uuid.New().String()
				types := []string{"coding_task", "design_task", "research_task"}

				for _, bType := range types {
					bounty := Bounty{
						OwnerID:     person.OwnerPubKey,
						Title:       fmt.Sprintf("Bounty %s", bType),
						Description: fmt.Sprintf("Test bounty for %s", bType),
						Price:       1000,
						Type:        bType,
						PhaseUuid:   &phaseUuid,
						Created:     currentTime,
					}
					TestDB.db.Create(&bounty)
				}

				return phaseUuid
			},
			expectedCount: 3,
			validate: func(t *testing.T, bounties []Bounty) {
				assert.Len(t, bounties, 3)
				types := make(map[string]bool)
				for _, b := range bounties {
					types[b.Type] = true
				}
				assert.Len(t, types, 3)
			},
		},
		{
			name: "Unicode characters in bounty titles",
			setup: func() string {
				phaseUuid := uuid.New().String()
				bounty := Bounty{
					OwnerID:     person.OwnerPubKey,
					Title:       "测试赏金 テストバウンティ",
					Description: "Unicode test bounty",
					Price:       1000,
					Type:        "coding_task",
					PhaseUuid:   &phaseUuid,
					Created:     currentTime,
				}
				TestDB.db.Create(&bounty)
				return phaseUuid
			},
			expectedCount: 1,
			validate: func(t *testing.T, bounties []Bounty) {
				assert.Len(t, bounties, 1)
				assert.Equal(t, "测试赏金 テストバウンティ", bounties[0].Title)
			},
		},
		{
			name: "Valid Phase UUID with Multiple Bounties (Different IDs)",
			setup: func() string {
				phaseUuid := uuid.New().String()
				for i := 1; i <= 5; i++ {
					bounty := Bounty{
						ID:          uint(i),
						OwnerID:     person.OwnerPubKey,
						Title:       fmt.Sprintf("Bounty %d", i),
						Description: fmt.Sprintf("Test bounty %d", i),
						Price:       uint(i * 1000),
						Type:        "coding_task",
						PhaseUuid:   &phaseUuid,
						Created:     currentTime,
					}
					TestDB.db.Create(&bounty)
				}
				return phaseUuid
			},
			expectedCount: 5,
			validate: func(t *testing.T, bounties []Bounty) {
				assert.Len(t, bounties, 5)
				for i, b := range bounties {
					assert.Equal(t, uint(i+1), b.ID)
				}
			},
		},
		{
			name: "Phase UUID with Special Characters",
			setup: func() string {
				phaseUuid := "special!@#$%^&*()"
				bounty := Bounty{
					ID:          1,
					OwnerID:     person.OwnerPubKey,
					Title:       "Special Chars Test",
					Description: "Test bounty with special chars in UUID",
					Price:       1000,
					Type:        "coding_task",
					PhaseUuid:   &phaseUuid,
					Created:     currentTime,
				}
				TestDB.db.Create(&bounty)
				return phaseUuid
			},
			expectedCount: 1,
			validate: func(t *testing.T, bounties []Bounty) {
				assert.Len(t, bounties, 1)
				assert.Equal(t, "Special Chars Test", bounties[0].Title)
			},
		},
		{
			name: "Non-Existent Phase UUID",
			setup: func() string {
				return uuid.New().String()
			},
			expectedCount: 0,
			validate: func(t *testing.T, bounties []Bounty) {
				assert.Empty(t, bounties)
			},
		},
		{
			name: "Phase UUID with SQL Injection Attempt",
			setup: func() string {
				phaseUuid := "'; DROP TABLE bounties; --"
				bounty := Bounty{
					ID:          1,
					OwnerID:     person.OwnerPubKey,
					Title:       "SQL Injection Test",
					Description: "Test bounty with SQL injection attempt",
					Price:       1000,
					Type:        "coding_task",
					PhaseUuid:   &phaseUuid,
					Created:     currentTime,
				}
				TestDB.db.Create(&bounty)
				return phaseUuid
			},
			expectedCount: 1,
			validate: func(t *testing.T, bounties []Bounty) {
				assert.Len(t, bounties, 1)
				assert.Equal(t, "SQL Injection Test", bounties[0].Title)
			},
		},
		{
			name: "Null Phase UUID",
			setup: func() string {
				bounty := Bounty{
					ID:          1,
					OwnerID:     person.OwnerPubKey,
					Title:       "Null UUID Test",
					Description: "Test bounty with null UUID",
					Price:       1000,
					Type:        "coding_task",
					PhaseUuid:   nil,
					Created:     currentTime,
				}
				TestDB.db.Create(&bounty)
				return ""
			},
			expectedCount: 0,
			validate: func(t *testing.T, bounties []Bounty) {
				assert.Empty(t, bounties)
			},
		},
		{
			name: "Maximum Length Phase UUID",
			setup: func() string {
				phaseUuid := strings.Repeat("a", 255)
				bounty := Bounty{
					ID:          1,
					OwnerID:     person.OwnerPubKey,
					Title:       "Max Length UUID Test",
					Description: "Test bounty with maximum length UUID",
					Price:       1000,
					Type:        "coding_task",
					PhaseUuid:   &phaseUuid,
					Created:     currentTime,
				}
				TestDB.db.Create(&bounty)
				return phaseUuid
			},
			expectedCount: 1,
			validate: func(t *testing.T, bounties []Bounty) {
				assert.Len(t, bounties, 1)
				assert.Equal(t, "Max Length UUID Test", bounties[0].Title)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			TestDB.DeleteAllBounties()

			phaseUuid := tt.setup()

			bounties := TestDB.GetBountiesByPhaseUuid(phaseUuid)

			assert.Equal(t, tt.expectedCount, len(bounties))
			if tt.validate != nil {
				tt.validate(t, bounties)
			}
		})
	}
}