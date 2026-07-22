package db

import (
	"testing"
)

func TestCopyFood(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	userA := createTestUser(t)
	userB := createTestUser(t)
	flour := createTestIngredient(t, userA, "Lineage Flour")

	extID := "fdc_lineage_test"
	barcode := "1234567890"
	source, err := CreateFood(Food{
		CreatorID:         userA.ID,
		Name:              "Pancakes",
		Calories:          200,
		Protein:           6,
		Carbs:             30,
		Fat:               5,
		MeasurementUnit:   "g",
		MeasurementAmount: 100,
		ExternalID:        &extID,
		Barcode:           &barcode,
		Nutrients:         []FoodNutrient{{Name: "Iron", Amount: 2, Unit: "mg"}},
		Portions:          []FoodPortion{{Name: "1 stack", Amount: 1, GramWeight: 150}},
		Ingredients:       []RecipeItems{{IngredientID: flour.ID, Amount: 50}},
	})
	if err != nil {
		t.Fatalf("CreateFood failed: %v", err)
	}

	copied, err := CopyFood(source.ID, userB.ID)
	if err != nil {
		t.Fatalf("CopyFood failed: %v", err)
	}

	if copied.ID == source.ID {
		t.Errorf("Copy should have a new id")
	}
	if copied.FamilyID == source.FamilyID {
		t.Errorf("Copy should start a new family")
	}
	if FoodID(copied.FamilyID) != copied.ID {
		t.Errorf("Copy's family_id should equal its own id")
	}
	if copied.Version != 1 || !copied.IsCurrent {
		t.Errorf("Copy should be version 1 and current, got v%d current=%v", copied.Version, copied.IsCurrent)
	}
	if copied.CreatorID != userB.ID {
		t.Errorf("Copy should belong to the copying user")
	}
	if copied.CopiedFromID == nil || *copied.CopiedFromID != source.ID {
		t.Errorf("Copy should record copied_from_id = source id")
	}
	if copied.Public != source.Public {
		t.Errorf("Copy should inherit the source's visibility")
	}
	if copied.ExternalID != nil || copied.Barcode != nil {
		t.Errorf("Copy should clear external_id and barcode, got %v / %v", copied.ExternalID, copied.Barcode)
	}

	// Relations copied; recipe items reference the same ingredient.
	fetched, err := GetFood(copied.ID)
	if err != nil {
		t.Fatalf("GetFood(copy) failed: %v", err)
	}
	if fetched.CopiedFromID == nil || *fetched.CopiedFromID != source.ID {
		t.Errorf("Fetched copy should retain copied_from_id")
	}
	if len(fetched.Nutrients) != 1 || fetched.Nutrients[0].Name != "Iron" {
		t.Errorf("Copy should have source's nutrients, got %v", fetched.Nutrients)
	}
	if len(fetched.Portions) != 1 || fetched.Portions[0].GramWeight != 150 {
		t.Errorf("Copy should have source's portions, got %v", fetched.Portions)
	}
	if len(fetched.Ingredients) != 1 || fetched.Ingredients[0].IngredientID != flour.ID {
		t.Errorf("Copy should reference the same ingredients, got %v", fetched.Ingredients)
	}

	// Source untouched.
	sourceAfter, err := GetFood(source.ID)
	if err != nil || sourceAfter == nil {
		t.Fatalf("GetFood(source) failed: %v", err)
	}
	if sourceAfter.CopiedFromID != nil {
		t.Errorf("Source should have no copied_from_id")
	}
	if !sourceAfter.IsCurrent || sourceAfter.ExternalID == nil {
		t.Errorf("Source should be unchanged by the copy")
	}
}

func TestCopyFood_DeletedSource(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	source, err := CreateFood(Food{CreatorID: user.ID, Name: "Doomed"})
	if err != nil {
		t.Fatalf("CreateFood failed: %v", err)
	}
	if err := DeleteFood(source.ID); err != nil {
		t.Fatalf("DeleteFood failed: %v", err)
	}
	if _, err := CopyFood(source.ID, user.ID); err == nil {
		t.Errorf("CopyFood of a deleted source should fail")
	}
}

func TestCopyFood_LineageCarriedForwardOnUpdate(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	source, err := CreateFood(Food{CreatorID: user.ID, Name: "Original"})
	if err != nil {
		t.Fatalf("CreateFood failed: %v", err)
	}
	copied, err := CopyFood(source.ID, user.ID)
	if err != nil {
		t.Fatalf("CopyFood failed: %v", err)
	}

	// Editing the copy must not lose (or allow rewriting) its lineage.
	edit := *copied
	edit.Name = "Renamed Copy"
	edit.CopiedFromID = nil // simulate an API update payload that omits lineage
	updated, err := UpdateFood(copied.ID, edit)
	if err != nil {
		t.Fatalf("UpdateFood failed: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("Expected version 2, got %d", updated.Version)
	}
	if updated.CopiedFromID == nil || *updated.CopiedFromID != source.ID {
		t.Errorf("copied_from_id should be carried forward on update")
	}
}

func TestCopyFood_IndependentVersionHistories(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	a, err := CreateFood(Food{CreatorID: user.ID, Name: "Food A"})
	if err != nil {
		t.Fatalf("CreateFood failed: %v", err)
	}
	b, err := CopyFood(a.ID, user.ID)
	if err != nil {
		t.Fatalf("CopyFood failed: %v", err)
	}

	// Update A twice and B once; histories must evolve independently.
	editA := *a
	editA.Name = "Food A v2"
	a2, err := UpdateFood(a.ID, editA)
	if err != nil {
		t.Fatalf("UpdateFood(A) failed: %v", err)
	}
	editA2 := *a2
	editA2.Name = "Food A v3"
	a3, err := UpdateFood(a2.ID, editA2)
	if err != nil {
		t.Fatalf("UpdateFood(A v2) failed: %v", err)
	}
	editB := *b
	editB.Name = "Food B v2"
	b2, err := UpdateFood(b.ID, editB)
	if err != nil {
		t.Fatalf("UpdateFood(B) failed: %v", err)
	}

	aVersions, err := GetFoodVersions(a3.ID)
	if err != nil {
		t.Fatalf("GetFoodVersions(A) failed: %v", err)
	}
	if len(aVersions) != 3 {
		t.Fatalf("Expected 3 versions of A, got %d", len(aVersions))
	}
	bVersions, err := GetFoodVersions(b2.ID)
	if err != nil {
		t.Fatalf("GetFoodVersions(B) failed: %v", err)
	}
	if len(bVersions) != 2 {
		t.Fatalf("Expected 2 versions of B, got %d", len(bVersions))
	}
	for _, v := range aVersions {
		if v.FamilyID != a.FamilyID {
			t.Errorf("A's history contains foreign family %v", v.FamilyID)
		}
		if (v.Version == 3) != v.IsCurrent {
			t.Errorf("A: only v3 should be current (v%d current=%v)", v.Version, v.IsCurrent)
		}
	}
	for _, v := range bVersions {
		if v.FamilyID != b.FamilyID {
			t.Errorf("B's history contains foreign family %v", v.FamilyID)
		}
		if (v.Version == 2) != v.IsCurrent {
			t.Errorf("B: only v2 should be current (v%d current=%v)", v.Version, v.IsCurrent)
		}
	}

	// B's lineage stays pinned to the version of A it was copied from.
	if b2.CopiedFromID == nil || *b2.CopiedFromID != a.ID {
		t.Errorf("B should stay pinned to A's original version despite A's updates")
	}

	// Deleting A only touches A's family; B stays live and lineage resolves.
	if err := DeleteFood(a3.ID); err != nil {
		t.Fatalf("DeleteFood(A) failed: %v", err)
	}
	bAfter, err := GetFood(b2.ID)
	if err != nil {
		t.Fatalf("GetFood(B) after deleting A failed: %v", err)
	}
	if bAfter == nil || bAfter.DeletedAt != nil {
		t.Fatalf("B should remain live after A's deletion")
	}
	lineage, err := GetFoodLineage(b2.ID, user.ID)
	if err != nil {
		t.Fatalf("GetFoodLineage(B) failed: %v", err)
	}
	if len(lineage.Ancestors) != 1 {
		t.Fatalf("Expected 1 ancestor for B, got %d", len(lineage.Ancestors))
	}
	if !lineage.Ancestors[0].Deleted {
		t.Errorf("A's ancestor node should be flagged deleted")
	}
	if lineage.Tree == nil || !lineage.Tree.Deleted {
		t.Errorf("Tree root (A) should be flagged deleted")
	}
}

func TestGetFoodLineage(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	a, err := CreateFood(Food{CreatorID: user.ID, Name: "Root A"})
	if err != nil {
		t.Fatalf("CreateFood failed: %v", err)
	}
	b, err := CopyFood(a.ID, user.ID) // A -> B
	if err != nil {
		t.Fatalf("CopyFood(A->B) failed: %v", err)
	}
	c, err := CopyFood(b.ID, user.ID) // B -> C
	if err != nil {
		t.Fatalf("CopyFood(B->C) failed: %v", err)
	}
	d, err := CopyFood(a.ID, user.ID) // A -> D (sibling branch)
	if err != nil {
		t.Fatalf("CopyFood(A->D) failed: %v", err)
	}

	// Update A so its current version differs from the version B/D were copied
	// from, then copy from the new current version: E must join the same tree.
	editA := *a
	editA.Name = "Root A v2"
	a2, err := UpdateFood(a.ID, editA)
	if err != nil {
		t.Fatalf("UpdateFood(A) failed: %v", err)
	}
	e, err := CopyFood(a2.ID, user.ID) // A(v2) -> E
	if err != nil {
		t.Fatalf("CopyFood(A v2->E) failed: %v", err)
	}

	// Ancestors of C: [B, A] nearest-first, version-pinned.
	lineage, err := GetFoodLineage(c.ID, user.ID)
	if err != nil {
		t.Fatalf("GetFoodLineage(C) failed: %v", err)
	}
	if len(lineage.Ancestors) != 2 {
		t.Fatalf("Expected 2 ancestors for C, got %d", len(lineage.Ancestors))
	}
	if lineage.Ancestors[0].FoodID != b.ID {
		t.Errorf("First ancestor should be B's pinned version")
	}
	if lineage.Ancestors[1].FoodID != a.ID {
		t.Errorf("Second ancestor should be A's pinned (v1) version")
	}
	if lineage.Ancestors[0].Food == nil || lineage.Ancestors[1].Food == nil {
		t.Errorf("Ancestors visible to the requester should be hydrated")
	}

	// Tree rooted at A's family: children {B, D, E}; B has child C.
	tree := lineage.Tree
	if tree == nil {
		t.Fatal("Expected a lineage tree")
	}
	if tree.FamilyID != a.FamilyID {
		t.Errorf("Tree root should be A's family")
	}
	if tree.Food == nil || tree.Food.ID != a2.ID {
		t.Errorf("Tree root should be hydrated with A's current version")
	}
	if len(tree.Children) != 3 {
		t.Fatalf("Expected 3 children of root (B, D, E), got %d", len(tree.Children))
	}
	childByFamily := map[FoodFamilyID]*FoodLineageNode{}
	for _, ch := range tree.Children {
		childByFamily[ch.FamilyID] = ch
	}
	bNode := childByFamily[b.FamilyID]
	if bNode == nil {
		t.Fatal("B missing from tree")
	}
	if childByFamily[d.FamilyID] == nil || childByFamily[e.FamilyID] == nil {
		t.Fatal("D or E missing from tree")
	}
	if len(bNode.Children) != 1 || bNode.Children[0].FamilyID != c.FamilyID {
		t.Errorf("B should have exactly child C")
	}

	// Lineage viewed from a leaf and from the root agree on the tree.
	fromRoot, err := GetFoodLineage(a2.ID, user.ID)
	if err != nil {
		t.Fatalf("GetFoodLineage(A) failed: %v", err)
	}
	if len(fromRoot.Ancestors) != 0 {
		t.Errorf("Root should have no ancestors, got %d", len(fromRoot.Ancestors))
	}
	if fromRoot.Tree == nil || fromRoot.Tree.FamilyID != a.FamilyID || len(fromRoot.Tree.Children) != 3 {
		t.Errorf("Tree from root should match tree from leaf")
	}
}

func TestGetFoodLineage_RedactsInvisibleFoods(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	owner := createTestUser(t)
	other := createTestUser(t)

	// A private (non-public) food owned by `owner`, copied by `owner`.
	private, err := CreateFood(Food{CreatorID: owner.ID, Name: "Secret Sauce", Public: false})
	if err != nil {
		t.Fatalf("CreateFood failed: %v", err)
	}
	copied, err := CopyFood(private.ID, owner.ID)
	if err != nil {
		t.Fatalf("CopyFood failed: %v", err)
	}

	// The owner sees everything.
	asOwner, err := GetFoodLineage(copied.ID, owner.ID)
	if err != nil {
		t.Fatalf("GetFoodLineage(owner) failed: %v", err)
	}
	if asOwner.Ancestors[0].Redacted || asOwner.Ancestors[0].Food == nil {
		t.Errorf("Owner should see the private ancestor")
	}

	// Another user gets a redacted stub in its place; topology intact.
	asOther, err := GetFoodLineage(copied.ID, other.ID)
	if err != nil {
		t.Fatalf("GetFoodLineage(other) failed: %v", err)
	}
	if len(asOther.Ancestors) != 1 {
		t.Fatalf("Topology should be preserved for other users")
	}
	if !asOther.Ancestors[0].Redacted || asOther.Ancestors[0].Food != nil {
		t.Errorf("Private ancestor should be redacted for other users")
	}
	if asOther.Tree == nil || !asOther.Tree.Redacted {
		t.Errorf("Private tree root should be redacted for other users")
	}
}
