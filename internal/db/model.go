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
	ID                  UserID     `json:"id"`
	Name                string     `json:"name"`
	Email               string     `json:"email"`
	DisabledAt          *time.Time `json:"disabled_at"`
	CalorieGoal         *int       `json:"calorie_goal"`
	ClownMode           bool       `json:"clown_mode"`
	HidePublicUserFoods bool       `json:"hide_public_user_foods"`
	WeightGoal          *float64   `json:"weight_goal"`
	WeightUnit          string     `json:"weight_unit"`
	CreatedAt           time.Time  `json:"created_at"`
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
	Servings          float64        `json:"servings"`
	Public            bool           `json:"public"`
	ExternalID        *string        `json:"external_id"`
	BrandOwner        *string        `json:"brand_owner,omitempty"`
	Barcode           *string        `json:"barcode,omitempty"`
	IngredientsText   *string        `json:"ingredients_text,omitempty"`
	Category          *string        `json:"category,omitempty"`
	Ingredients       []RecipeItems  `json:"ingredients,omitempty"`
	Nutrients         []FoodNutrient `json:"nutrients,omitempty"`
	Portions          []FoodPortion  `json:"portions,omitempty"`
	CopiedFromID      *FoodID        `json:"copied_from_id,omitempty"` // Exact food version this food was copied from
	CreatedAt         time.Time      `json:"created_at"`
	DeletedAt         *time.Time     `json:"deleted_at"`
}

// FoodLineageNode is one node in a copy-lineage tree. Nodes represent food
// families (hydrated with the family's current version); edges follow the
// version-pinned copied_from_id references. Food is nil when the requester
// is not allowed to see it (Redacted), preserving tree topology.
type FoodLineageNode struct {
	FoodID   FoodID             `json:"food_id"`   // current version id (topology anchor)
	FamilyID FoodFamilyID       `json:"family_id"` // stable family identifier
	Food     *Food              `json:"food,omitempty"`
	Redacted bool               `json:"redacted,omitempty"`
	Deleted  bool               `json:"deleted,omitempty"`
	Children []*FoodLineageNode `json:"children"`
}

// FoodLineage is the full copy-lineage view for one food: the chain of foods
// it was copied from (nearest-first, version-pinned) and the whole copy tree
// rooted at the lineage's origin.
type FoodLineage struct {
	FoodID    FoodID             `json:"food_id"`
	FamilyID  FoodFamilyID       `json:"family_id"`
	Ancestors []*FoodLineageNode `json:"ancestors"`
	Tree      *FoodLineageNode   `json:"tree"`
}

// FoodPortions
//
//	food_id
//	name (e.g. '1 slice')
//	amount (e.g. 1)
//	unit (e.g. 'slice')
//	gram_weight (e.g. 28.0)
type FoodPortion struct {
	FoodID     FoodID  `json:"food_id"`
	Name       string  `json:"name"`
	Amount     float64 `json:"amount"`
	Unit       *string `json:"unit,omitempty"`
	GramWeight float64 `json:"gram_weight"`
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
	ID          FoodLogEntryID `json:"id"`
	UserID      UserID         `json:"user_id"`
	FoodID      *FoodID        `json:"food_id"`
	Food        *Food          `json:"food,omitempty"`
	PortionName *string        `json:"portion_name,omitempty"`
	Calories    *float64       `json:"calories"` // Nullable, used when FoodID is nil
	Protein   *float64       `json:"protein"`
	Carbs     *float64       `json:"carbs"`
	Fat       *float64       `json:"fat"`
	Amount    float64        `json:"amount"`
	MealTag   string         `json:"meal_tag"`
	Note         *string         `json:"note"`
	CopiedFromID *FoodLogEntryID `json:"copied_from_id,omitempty"` // Source entry when created via a day-copy
	LoggedAt     time.Time       `json:"logged_at"`
	CreatedAt    time.Time       `json:"created_at"`
	DeletedAt    *time.Time      `json:"deleted_at"`
}

// SQL Driver Support

func (id UserID) Value() (driver.Value, error) {
	if uuid.UUID(id) == uuid.Nil {
		return nil, nil
	}
	return uuid.UUID(id).Value()
}
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

// nullFoodID scans a nullable foods.id column into a *FoodID field:
// NULL becomes nil, otherwise the pointer is set to the scanned id.
type nullFoodID struct{ dest **FoodID }

func (n nullFoodID) Scan(src any) error {
	if src == nil {
		*n.dest = nil
		return nil
	}
	var id FoodID
	if err := id.Scan(src); err != nil {
		return err
	}
	*n.dest = &id
	return nil
}

// nullFoodLogEntryID is the food_log_entries.id counterpart of nullFoodID.
type nullFoodLogEntryID struct{ dest **FoodLogEntryID }

func (n nullFoodLogEntryID) Scan(src any) error {
	if src == nil {
		*n.dest = nil
		return nil
	}
	var id FoodLogEntryID
	if err := id.Scan(src); err != nil {
		return err
	}
	*n.dest = &id
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

// WeightLogs
//
//	id
//	user_id
//	weight
//	unit
//	logged_at
//	created_at
//	deleted_at
type WeightLogID uuid.UUID
type WeightLog struct {
	ID        WeightLogID `json:"id"`
	UserID    UserID      `json:"user_id"`
	Weight    float64     `json:"weight"`
	Unit      string      `json:"unit"`
	LoggedAt  time.Time   `json:"logged_at"`
	CreatedAt time.Time   `json:"created_at"`
	DeletedAt *time.Time  `json:"deleted_at"`
}

func (id WeightLogID) Value() (driver.Value, error) { return uuid.UUID(id).Value() }
func (id *WeightLogID) Scan(src any) error {
	var u uuid.UUID
	if err := u.Scan(src); err != nil {
		return err
	}
	*id = WeightLogID(u)
	return nil
}

func (id WeightLogID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID(id))
}
func (id *WeightLogID) UnmarshalJSON(data []byte) error {
	var u uuid.UUID
	if err := json.Unmarshal(data, &u); err != nil {
		return err
	}
	*id = WeightLogID(u)
	return nil
}
