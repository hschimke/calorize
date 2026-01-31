package db

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Users
//
//	id (UUID)
//	name
//	email (Unique)
//	disabled_at (Nullable)
//	created_at
type UserID uuid.UUID
type User struct {
	ID         UserID     `json:"id"`
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	DisabledAt *time.Time `json:"disabled_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// UserCredentials (WebAuthn)
//
//	id (Credential ID - WebAuthn raw bytes)
//	user_id
//	name (Device Name)
//	public_key
//	attestation_type
//	aaguid
//	sign_count
//	transports
//	backup_eligible (Boolean)
//	backup_state (Boolean)
//	created_at
//	last_used_at
type UserCredentialID []byte
type UserCredential struct {
	ID              UserCredentialID `json:"id"`
	UserID          UserID           `json:"user_id"`
	Name            string           `json:"name"`
	PublicKey       []byte           `json:"public_key"`
	AttestationType string           `json:"attestation_type"`
	AAGUID          string           `json:"aaguid"`
	SignCount       uint32           `json:"sign_count"`
	Transports      []string         `json:"transports"`
	BackupEligible  bool             `json:"backup_eligible"`
	BackupState     bool             `json:"backup_state"`
	CreatedAt       time.Time        `json:"created_at"`
	LastUsedAt      time.Time        `json:"last_used_at"`
}

// Foods (Versioned)
//
//	id (Version UUID)
//	family_id (UUID - links versions together)
//	version (Integer)
//	is_current (Boolean)
//	name
//	calories
//	protein
//	carbs
//	fat
//	type (Enum: 'food', 'recipe')
//	measurement_unit (e.g. 'g', 'ml', 'serving')
//	measurement_amount (e.g. 100)
//	created_at
//	deleted_at
type FoodID uuid.UUID
type FoodFamilyID uuid.UUID
type Food struct {
	ID                FoodID         `json:"id"`
	CreatorID         UserID         `json:"creator_id"`
	FamilyID          FoodFamilyID   `json:"family_id"`
	Version           int            `json:"version"`
	IsCurrent         bool           `json:"is_current"`
	Name              string         `json:"name"`
	Calories          float64        `json:"calories"`
	Protein           float64        `json:"protein"`
	Carbs             float64        `json:"carbs"`
	Fat               float64        `json:"fat"`
	Type              string         `json:"type"`
	MeasurementUnit   string         `json:"measurement_unit"`
	MeasurementAmount float64        `json:"measurement_amount"`
	Public            bool           `json:"public"`
	Ingredients       []RecipeItems  `json:"ingredients,omitempty"`
	Nutrients         []FoodNutrient `json:"nutrients,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	DeletedAt         *time.Time     `json:"deleted_at"`
}

// FoodNutrients (Micro-nutrients)
//
//	food_id
//	name (e.g. 'Vitamin C')
//	amount
//	unit (e.g. 'mg')
type FoodNutrient struct {
	FoodID FoodID  `json:"food_id"`
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

// RecipeItems (Join Table)
//
//	recipe_id (FK to foods.id)
//	ingredient_id (FK to foods.id)
//	amount
type RecipeID uuid.UUID
type RecipeItems struct {
	RecipeID     RecipeID `json:"recipe_id"`
	IngredientID FoodID   `json:"ingredient_id"`
	Amount       float64  `json:"amount"`
}

// Logs
//
//	id
//	user_id
//	food_id (Specific version)
//	amount
//	meal_tag (String: 'breakfast', 'lunch', etc.)
//	logged_at (Date/Time)
//	created_at
//	deleted_at
type FoodLogEntryID uuid.UUID
type FoodLogEntry struct {
	ID        FoodLogEntryID `json:"id"`
	UserID    UserID         `json:"user_id"`
	FoodID    FoodID         `json:"food_id"`
	Amount    float64        `json:"amount"`
	MealTag   string         `json:"meal_tag"`
	LoggedAt  time.Time      `json:"logged_at"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt *time.Time     `json:"deleted_at"`
}

// SQL Driver Support

func (id UserID) Value() (driver.Value, error) { return uuid.UUID(id).Value() }
func (id *UserID) Scan(src any) error {
	var u uuid.UUID
	if err := u.Scan(src); err != nil {
		return err
	}
	*id = UserID(u)
	return nil
}

func (id FoodID) Value() (driver.Value, error) { return uuid.UUID(id).Value() }
func (id *FoodID) Scan(src any) error {
	var u uuid.UUID
	if err := u.Scan(src); err != nil {
		return err
	}
	*id = FoodID(u)
	return nil
}

func (id FoodFamilyID) Value() (driver.Value, error) { return uuid.UUID(id).Value() }
func (id *FoodFamilyID) Scan(src any) error {
	var u uuid.UUID
	if err := u.Scan(src); err != nil {
		return err
	}
	*id = FoodFamilyID(u)
	return nil
}

func (id RecipeID) Value() (driver.Value, error) { return uuid.UUID(id).Value() }
func (id *RecipeID) Scan(src any) error {
	var u uuid.UUID
	if err := u.Scan(src); err != nil {
		return err
	}
	*id = RecipeID(u)
	return nil
}

func (id FoodLogEntryID) Value() (driver.Value, error) { return uuid.UUID(id).Value() }
func (id *FoodLogEntryID) Scan(src any) error {
	var u uuid.UUID
	if err := u.Scan(src); err != nil {
		return err
	}
	*id = FoodLogEntryID(u)
	return nil
}

// JSON Marshaling

func (id UserID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID(id))
}
func (id *UserID) UnmarshalJSON(data []byte) error {
	var u uuid.UUID
	if err := json.Unmarshal(data, &u); err != nil {
		return err
	}
	*id = UserID(u)
	return nil
}

func (id FoodID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID(id))
}
func (id *FoodID) UnmarshalJSON(data []byte) error {
	var u uuid.UUID
	if err := json.Unmarshal(data, &u); err != nil {
		return err
	}
	*id = FoodID(u)
	return nil
}

func (id FoodFamilyID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID(id))
}
func (id *FoodFamilyID) UnmarshalJSON(data []byte) error {
	var u uuid.UUID
	if err := json.Unmarshal(data, &u); err != nil {
		return err
	}
	*id = FoodFamilyID(u)
	return nil
}

func (id RecipeID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID(id))
}
func (id *RecipeID) UnmarshalJSON(data []byte) error {
	var u uuid.UUID
	if err := json.Unmarshal(data, &u); err != nil {
		return err
	}
	*id = RecipeID(u)
	return nil
}

func (id FoodLogEntryID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID(id))
}
func (id *FoodLogEntryID) UnmarshalJSON(data []byte) error {
	var u uuid.UUID
	if err := json.Unmarshal(data, &u); err != nil {
		return err
	}
	*id = FoodLogEntryID(u)
	return nil
}
