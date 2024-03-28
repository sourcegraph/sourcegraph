package users

import (
	"context"
	"time"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type UsersStatsDateTimeRange struct {
	Lte   *string
	Gte   *string
	Empty *bool
	Not   *bool
}

func (d *UsersStatsDateTimeRange) toSQLConds(column string) ([]*sqlf.Query, error) {
	conds := []*sqlf.Query{}

	if d.Empty != nil && *d.Empty {
		conds = append(conds, sqlf.Sprintf(column+" IS NULL"))
	} else {
		if d.Lte != nil {
			lte, err := time.Parse(time.RFC3339, *d.Lte)
			if err != nil {
				return nil, err
			}
			conds = append(conds, sqlf.Sprintf(column+" <= %s", lte))
		}
		if d.Gte != nil {
			gte, err := time.Parse(time.RFC3339, *d.Gte)
			if err != nil {
				return nil, err
			}
			conds = append(conds, sqlf.Sprintf(column+" >= %s", gte))
		}
	}

	if d.Not != nil && *d.Not {
		return []*sqlf.Query{sqlf.Sprintf("NOT (%s)", sqlf.Join(conds, "AND"))}, nil
	}
	return conds, nil
}

type UsersStatsNumberRange struct {
	Gte *float64
	Lte *float64
}

func (d *UsersStatsNumberRange) toSQLConds(column string) []*sqlf.Query {
	var conds []*sqlf.Query

	if d.Lte != nil {
		conds = append(conds, sqlf.Sprintf(column+" <= %s", d.Lte))
	}
	if d.Gte != nil {
		conds = append(conds, sqlf.Sprintf(column+" >= %s", d.Gte))
	}

	return conds
}

type UsersStatsFilters struct {
	Query        *string
	SiteAdmin    *bool
	Username     *string
	Email        *string
	LastActiveAt *UsersStatsDateTimeRange
	DeletedAt    *UsersStatsDateTimeRange
	CreatedAt    *UsersStatsDateTimeRange
	EventsCount  *UsersStatsNumberRange
}

type UsersStats struct {
	DB      database.DB
	Filters UsersStatsFilters
}

func (s *UsersStats) makeQueryParameters() ([]*sqlf.Query, error) {
	conds := []*sqlf.Query{sqlf.Sprintf("TRUE")}
	if s.Filters.Query != nil && *s.Filters.Query != "" {
		query := "%" + *s.Filters.Query + "%"
		conds = append(conds, sqlf.Sprintf("(username ILIKE %s OR display_name ILIKE %s OR primary_email ILIKE %s)", query, query, query))
	}
	if s.Filters.SiteAdmin != nil {
		// this is referring to the site_admin field returned from the CTE and not necessarily users.site_admin
		conds = append(conds, sqlf.Sprintf("site_admin = %s", *s.Filters.SiteAdmin))
	}
	if s.Filters.Username != nil {
		conds = append(conds, sqlf.Sprintf("username ILIKE %s", "%"+*s.Filters.Username+"%"))
	}
	if s.Filters.Email != nil {
		conds = append(conds, sqlf.Sprintf("primary_email ILIKE %s", "%"+*s.Filters.Email+"%"))
	}
	if s.Filters.DeletedAt != nil {
		deletedAtConds, err := s.Filters.DeletedAt.toSQLConds("deleted_at")
		if err != nil {
			return nil, err
		}
		conds = append(conds, deletedAtConds...)
	}

	if s.Filters.LastActiveAt != nil {
		lastActiveAtConds, err := s.Filters.LastActiveAt.toSQLConds("last_active_at")
		if err != nil {
			return nil, err
		}
		conds = append(conds, lastActiveAtConds...)
	}
	if s.Filters.CreatedAt != nil {
		createdAtConds, err := s.Filters.CreatedAt.toSQLConds("created_at")
		if err != nil {
			return nil, err
		}
		conds = append(conds, createdAtConds...)
	}

	if s.Filters.EventsCount != nil {
		eventsCountConds := s.Filters.EventsCount.toSQLConds("events_count")
		conds = append(conds, eventsCountConds...)
		if s.Filters.EventsCount.Lte != nil {
			conds = append(conds, sqlf.Sprintf("events_count <= %s", *s.Filters.EventsCount.Lte))
		}
		if s.Filters.EventsCount.Gte != nil {
			conds = append(conds, sqlf.Sprintf("events_count >= %s", *s.Filters.EventsCount.Gte))
		}
	}

	// Exclude Sourcegraph Operator user accounts
	conds = append(conds, sqlf.Sprintf(`
NOT EXISTS (
	SELECT FROM user_external_accounts
	WHERE
		service_type = 'sourcegraph-operator'
	AND user_id = aggregated_stats.id
)
`))
	return conds, nil
}

var (
	statsCTEQuery = `
	WITH aggregated_stats AS (
		SELECT
			users.id AS id,
			users.username,
			users.display_name,
			emails.email primary_email,
			users.created_at,
			stats.user_last_active_at AS last_active_at,
			users.deleted_at,
			is_user_site_admin(users.id) AS site_admin,
            (SELECT COUNT(user_id) FROM user_external_accounts WHERE user_id=users.id AND service_type = 'scim') >= 1 AS scim_controlled,
			COALESCE(stats.user_events_count, 0) AS events_count
		FROM users
			LEFT JOIN aggregated_user_statistics stats ON stats.user_id = users.id
			LEFT JOIN user_emails emails ON emails.user_id = users.id AND emails.is_primary = true
	)
	%s
	`
)

func (s *UsersStats) TotalCount(ctx context.Context) (float64, error) {
	var totalCount float64

	conds, err := s.makeQueryParameters()
	if err != nil {
		return 0, err
	}

	query := sqlf.Sprintf(statsCTEQuery, sqlf.Sprintf(`SELECT COUNT(id) FROM aggregated_stats WHERE %s`, sqlf.Join(conds, "AND")))
	if err := s.DB.QueryRowContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...).Scan(&totalCount); err != nil {
		return 0, err
	}

	return totalCount, nil
}

type UsersStatsListUsersFilters struct {
	OrderBy    *string
	Descending *bool
	Limit      *int32
	Offset     *int32
}

func (s *UsersStats) ListUsers(ctx context.Context, filters *UsersStatsListUsersFilters) ([]*UserStatItem, error) {
	// ORDER BY
	orderDirection := "ASC"
	if filters == nil {
		filters = &UsersStatsListUsersFilters{}
	}
	if filters.Descending != nil && *filters.Descending {
		orderDirection = "DESC"
	}
	orderBy := sqlf.Sprintf("id " + orderDirection)
	if filters.OrderBy != nil {
		newOrderColumn, err := toUsersField(*filters.OrderBy)
		orderBy = sqlf.Sprintf(newOrderColumn + " " + orderDirection)
		if err != nil {
			return nil, err
		}
	}

	// LIMIT
	limit := int32(100)
	if filters.Limit != nil {
		limit = *filters.Limit
	}

	// OFFSET
	offset := int32(0)
	if filters.Offset != nil {
		offset = *filters.Offset
	}

	conds, err := s.makeQueryParameters()
	if err != nil {
		return nil, err
	}

	query := sqlf.Sprintf(statsCTEQuery, sqlf.Sprintf(`
	SELECT id, username, display_name, primary_email, created_at, last_active_at, deleted_at, site_admin, scim_controlled, events_count FROM aggregated_stats WHERE %s ORDER BY %s NULLS LAST LIMIT %s OFFSET %s`, sqlf.Join(conds, "AND"), orderBy, limit, offset))

	rows, err := s.DB.QueryContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	nodes := make([]*UserStatItem, 0)
	for rows.Next() {
		var node UserStatItem

		if err := rows.Scan(&node.Id, &node.Username, &node.DisplayName, &node.PrimaryEmail, &node.CreatedAt, &node.LastActiveAt, &node.DeletedAt, &node.SiteAdmin, &node.SCIMControlled, &node.EventsCount); err != nil {
			return nil, err
		}

		nodes = append(nodes, &node)
	}

	return nodes, nil
}

func toUsersField(orderBy string) (string, error) {
	switch orderBy {
	case "USERNAME":
		return "username", nil
	case "EMAIL":
		return "primary_email", nil
	case "CREATED_AT":
		return "created_at", nil
	case "LAST_ACTIVE_AT":
		return "last_active_at", nil
	case "DELETED_AT":
		return "deleted_at", nil
	case "EVENTS_COUNT":
		return "events_count", nil
	case "SITE_ADMIN":
		return "site_admin", nil
	default:
		return "", errors.New("invalid orderBy")
	}
}

type UserStatItem struct {
	Id             int32
	Username       string
	DisplayName    *string
	PrimaryEmail   *string
	CreatedAt      time.Time
	LastActiveAt   *time.Time
	DeletedAt      *time.Time
	SiteAdmin      bool
	SCIMControlled bool
	EventsCount    float64
}
