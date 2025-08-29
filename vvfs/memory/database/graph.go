package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/apptype"
)

// GetRelationsForEntities returns relations touching provided entities
func (dm *DBManager) GetRelationsForEntities(ctx context.Context, projectName string, entities []apptype.Entity) ([]apptype.Relation, error) {
	if len(entities) == 0 {
		return []apptype.Relation{}, nil
	}
	db, err := dm.getDB(projectName)
	if err != nil {
		return nil, err
	}
	// Build IN clause dynamically
	names := make([]interface{}, len(entities))
	placeholders := make([]string, len(entities))
	for i, e := range entities {
		names[i] = e.Name
		placeholders[i] = "?"
	}
	query := fmt.Sprintf("SELECT source, target, relation_type FROM relations WHERE source IN (%s) OR target IN (%s)",
		strings.Join(placeholders, ","), strings.Join(placeholders, ","))
	args := append(names, names...)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []apptype.Relation
	for rows.Next() {
		var r apptype.Relation
		if err := rows.Scan(&r.From, &r.To, &r.RelationType); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
