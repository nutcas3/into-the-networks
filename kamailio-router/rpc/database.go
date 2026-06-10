package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
}

func NewDB(host string, port int, user, password, dbname string) (*DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &DB{DB: db}, nil
}

type Destination struct {
	ID          int    `json:"id"`
	SetID       int    `json:"setid"`
	Destination string `json:"destination"`
	Flags       int    `json:"flags"`
	Priority    int    `json:"priority"`
	Attrs       string `json:"attrs,omitempty"`
	Description string `json:"description,omitempty"`
}

type Carrier struct {
	ID          int    `json:"id"`
	CarrierID   int    `json:"carrierid"`
	CarrierName string `json:"carrier_name"`
	GWList      string `json:"gwlist"`
	Description string `json:"description,omitempty"`
}

type Route struct {
	RuleID   int    `json:"ruleid"`
	GroupID  int    `json:"groupid"`
	Prefix   string `json:"prefix"`
	Priority int    `json:"priority"`
	RouteID  int    `json:"routeid"`
	GWList   string `json:"gwlist"`
}

type HealthCheck struct {
	ID           int       `json:"id"`
	Destination  string    `json:"destination"`
	Status       string    `json:"status"`
	LastCheck    time.Time `json:"last_check"`
	ResponseTime int       `json:"response_time"`
	FailureCount int       `json:"failure_count"`
	SuccessCount int       `json:"success_count"`
}

func (db *DB) AddDestination(dest *Destination) error {
	query := `
		INSERT INTO dispatcher (setid, destination, flags, priority, attrs, description)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	return db.QueryRow(query, dest.SetID, dest.Destination, dest.Flags, dest.Priority, dest.Attrs, dest.Description).Scan(&dest.ID)
}

func (db *DB) RemoveDestination(setid int, destination string) error {
	query := `DELETE FROM dispatcher WHERE setid = $1 AND destination = $2`
	_, err := db.Exec(query, setid, destination)
	return err
}

func (db *DB) ListDestinations(setid int) ([]Destination, error) {
	query := `SELECT id, setid, destination, flags, priority, attrs, description FROM dispatcher WHERE setid = $1`
	rows, err := db.Query(query, setid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var destinations []Destination
	for rows.Next() {
		var dest Destination
		err := rows.Scan(&dest.ID, &dest.SetID, &dest.Destination, &dest.Flags, &dest.Priority, &dest.Attrs, &dest.Description)
		if err != nil {
			return nil, err
		}
		destinations = append(destinations, dest)
	}

	return destinations, nil
}

func (db *DB) AddCarrier(carrier *Carrier) error {
	query := `
		INSERT INTO dr_carriers (carrierid, carrier_name, gwlist, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	return db.QueryRow(query, carrier.CarrierID, carrier.CarrierName, carrier.GWList, carrier.Description).Scan(&carrier.ID)
}

func (db *DB) RemoveCarrier(carrierid int) error {
	query := `DELETE FROM dr_carriers WHERE carrierid = $1`
	_, err := db.Exec(query, carrierid)
	return err
}

func (db *DB) ListCarriers() ([]Carrier, error) {
	query := `SELECT id, carrierid, carrier_name, gwlist, description FROM dr_carriers`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var carriers []Carrier
	for rows.Next() {
		var carrier Carrier
		err := rows.Scan(&carrier.ID, &carrier.CarrierID, &carrier.CarrierName, &carrier.GWList, &carrier.Description)
		if err != nil {
			return nil, err
		}
		carriers = append(carriers, carrier)
	}

	return carriers, nil
}

func (db *DB) AddRoute(route *Route) error {
	query := `
		INSERT INTO dr_rules (groupid, prefix, priority, routeid, gwlist)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ruleid
	`
	return db.QueryRow(query, route.GroupID, route.Prefix, route.Priority, route.RouteID, route.GWList).Scan(&route.RuleID)
}

func (db *DB) GetRoute(prefix string) (*Route, error) {
	query := `
		SELECT ruleid, groupid, prefix, priority, routeid, gwlist
		FROM dr_rules
		WHERE prefix = $1
		ORDER BY priority ASC
		LIMIT 1
	`
	var route Route
	err := db.QueryRow(query, prefix).Scan(&route.RuleID, &route.GroupID, &route.Prefix, &route.Priority, &route.RouteID, &route.GWList)
	if err != nil {
		return nil, err
	}
	return &route, nil
}

func (db *DB) UpdateHealthCheck(destination string, status string, responseTime int) error {
	query := `
		INSERT INTO health_checks (destination, status, response_time, last_check)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (destination) DO UPDATE
		SET status = $2, response_time = $3, last_check = NOW(),
			failure_count = CASE WHEN $2 = 'healthy' THEN 0 ELSE failure_count + 1 END,
			success_count = CASE WHEN $2 = 'healthy' THEN success_count + 1 ELSE success_count END
	`
	_, err := db.Exec(query, destination, status, responseTime)
	return err
}

func (db *DB) GetHealthStatus() ([]HealthCheck, error) {
	query := `SELECT id, destination, status, last_check, response_time, failure_count, success_count FROM health_checks`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []HealthCheck
	for rows.Next() {
		var check HealthCheck
		err := rows.Scan(&check.ID, &check.Destination, &check.Status, &check.LastCheck, &check.ResponseTime, &check.FailureCount, &check.SuccessCount)
		if err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}

	return checks, nil
}
