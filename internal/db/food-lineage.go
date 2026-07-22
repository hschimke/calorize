package db

import (
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// maxLineageDepth caps recursive lineage walks. Organic cycles are impossible
// (copied_from_id is written once at creation and never modified), so this is
// purely a guard against pathological data.
const maxLineageDepth = 64

// foodVisibleTo reports whether the requester may see a food's details:
// system foods (no creator), public foods, and the requester's own foods.
func foodVisibleTo(f *Food, requester UserID) bool {
	return f.CreatorID == UserID(uuid.Nil) || f.Public || f.CreatorID == requester
}

// GetFoodLineage returns the copy lineage for a food: the version-pinned
// chain of foods it was copied from (nearest-first) and the full copy tree
// rooted at the lineage's origin family. Foods the requester may not see are
// included as redacted stubs so tree topology is preserved; soft-deleted
// foods are included and flagged.
func GetFoodLineage(foodID FoodID, requesterID UserID) (*FoodLineage, error) {
	// Resolve the requested version's family.
	var familyID FoodFamilyID
	err := db.QueryRow("SELECT family_id FROM foods WHERE id = ?", foodID).Scan(&familyID)
	if err != nil {
		return nil, fmt.Errorf("resolving food family: %w", err)
	}

	// Walk the copied_from chain upward from the requested version.
	ancestorIDs, err := getAncestorIDs(foodID)
	if err != nil {
		return nil, err
	}

	// The tree is rooted at the deepest ancestor's family (or our own family
	// when this food is not a copy).
	rootID := foodID
	if len(ancestorIDs) > 0 {
		rootID = ancestorIDs[len(ancestorIDs)-1]
	}
	var rootFamilyID FoodFamilyID
	if err := db.QueryRow("SELECT family_id FROM foods WHERE id = ?", rootID).Scan(&rootFamilyID); err != nil {
		return nil, fmt.Errorf("resolving root family: %w", err)
	}

	tree, treeFoodIDs, err := getLineageTreeRows(rootFamilyID)
	if err != nil {
		return nil, err
	}

	// Hydrate every food referenced by the ancestors and the tree in one batch.
	allIDs := make([]FoodID, 0, len(ancestorIDs)+len(treeFoodIDs))
	allIDs = append(allIDs, ancestorIDs...)
	allIDs = append(allIDs, treeFoodIDs...)
	foodMap, err := GetFoodsByIDs(allIDs)
	if err != nil {
		return nil, err
	}

	ancestors := make([]*FoodLineageNode, 0, len(ancestorIDs))
	for _, id := range ancestorIDs {
		ancestors = append(ancestors, newLineageNode(id, foodMap[id], requesterID))
	}

	root := buildLineageTree(tree, foodMap, requesterID)

	return &FoodLineage{
		FoodID:    foodID,
		FamilyID:  familyID,
		Ancestors: ancestors,
		Tree:      root,
	}, nil
}

// getAncestorIDs walks copied_from_id upward from the given version and
// returns the ancestor version ids nearest-first (excluding the food itself).
func getAncestorIDs(foodID FoodID) ([]FoodID, error) {
	query := `
		WITH RECURSIVE anc(id, copied_from_id, depth) AS (
			SELECT id, copied_from_id, 0 FROM foods WHERE id = ?
			UNION ALL
			SELECT f.id, f.copied_from_id, anc.depth + 1
			FROM foods f
			JOIN anc ON f.id = anc.copied_from_id
			WHERE anc.depth < ?
		)
		SELECT id FROM anc WHERE depth > 0 ORDER BY depth
	`
	rows, err := db.Query(query, foodID, maxLineageDepth)
	if err != nil {
		return nil, fmt.Errorf("walking food ancestors: %w", err)
	}
	defer rows.Close()

	var ids []FoodID
	for rows.Next() {
		var id FoodID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning ancestor id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating ancestors: %w", err)
	}
	return ids, nil
}

// lineageTreeRow is one family in the copy tree: the family's current
// version id and the family it was copied from (nil for the root).
type lineageTreeRow struct {
	id             FoodID
	familyID       FoodFamilyID
	parentFamilyID *FoodFamilyID
}

// getLineageTreeRows expands the copy tree downward from the root family.
// Tree nodes are families, represented by their current version; edges match
// a copy's version-pinned copied_from_id to any version of a parent family
// already in the tree, so copies made from old versions are still included.
func getLineageTreeRows(rootFamilyID FoodFamilyID) ([]lineageTreeRow, []FoodID, error) {
	query := `
		WITH RECURSIVE tree(id, family_id, parent_family_id, depth) AS (
			SELECT id, family_id, NULL, 0
			FROM foods WHERE family_id = ? AND is_current = true
			UNION ALL
			SELECT c.id, c.family_id, parent.family_id, t.depth + 1
			FROM foods c
			JOIN foods parent ON c.copied_from_id = parent.id
			JOIN tree t ON parent.family_id = t.family_id
			WHERE c.is_current = true AND t.depth < ?
		)
		SELECT id, family_id, parent_family_id FROM tree
	`
	rows, err := db.Query(query, rootFamilyID, maxLineageDepth)
	if err != nil {
		return nil, nil, fmt.Errorf("expanding copy tree: %w", err)
	}
	defer rows.Close()

	var treeRows []lineageTreeRow
	var ids []FoodID
	for rows.Next() {
		var row lineageTreeRow
		var parent uuid.NullUUID
		if err := rows.Scan(&row.id, &row.familyID, &parent); err != nil {
			return nil, nil, fmt.Errorf("scanning tree row: %w", err)
		}
		if parent.Valid {
			pf := FoodFamilyID(parent.UUID)
			row.parentFamilyID = &pf
		}
		treeRows = append(treeRows, row)
		ids = append(ids, row.id)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating tree rows: %w", err)
	}
	return treeRows, ids, nil
}

// newLineageNode wraps a hydrated food (or its absence) in a lineage node,
// applying visibility redaction and the deleted flag.
func newLineageNode(id FoodID, f *Food, requesterID UserID) *FoodLineageNode {
	node := &FoodLineageNode{FoodID: id, Children: []*FoodLineageNode{}}
	if f == nil {
		node.Redacted = true
		return node
	}
	node.FamilyID = f.FamilyID
	node.Deleted = f.DeletedAt != nil
	if foodVisibleTo(f, requesterID) {
		node.Food = f
	} else {
		node.Redacted = true
	}
	return node
}

// buildLineageTree assembles parent/child node links from the flat CTE rows.
func buildLineageTree(rows []lineageTreeRow, foodMap map[FoodID]*Food, requesterID UserID) *FoodLineageNode {
	if len(rows) == 0 {
		return nil
	}

	nodes := make(map[FoodFamilyID]*FoodLineageNode, len(rows))
	var root *FoodLineageNode
	for _, row := range rows {
		node := newLineageNode(row.id, foodMap[row.id], requesterID)
		node.FamilyID = row.familyID
		nodes[row.familyID] = node
		if row.parentFamilyID == nil {
			root = node
		}
	}
	for _, row := range rows {
		if row.parentFamilyID == nil {
			continue
		}
		if parent := nodes[*row.parentFamilyID]; parent != nil {
			parent.Children = append(parent.Children, nodes[row.familyID])
		}
	}
	// Stable child ordering: oldest copy first. A family_id equals the family's
	// v1 id, and UUIDv7 ids are time-ordered, so family_id order is creation order.
	for _, node := range nodes {
		sort.Slice(node.Children, func(i, j int) bool {
			return uuid.UUID(node.Children[i].FamilyID).String() < uuid.UUID(node.Children[j].FamilyID).String()
		})
	}
	return root
}
