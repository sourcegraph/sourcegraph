package lsifstore

import (
	"database/sql"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
)

type QualifiedDocumentData struct {
	UploadID int
	semantic.KeyedDocumentData
}

// scanDocumentData reads qualified document data from the given row object.
func (s *Store) scanDocumentData(rows *sql.Rows, queryErr error) (_ []QualifiedDocumentData, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	var values []QualifiedDocumentData
	for rows.Next() {
		record, err := s.scanSingleDocumentDataObject(rows)
		if err != nil {
			return nil, err
		}

		values = append(values, record)
	}

	return values, nil
}

// makeDocumentVisitor returns a function that accepts a mapping function, reads
// document values from the given row object and calls the mapping function on each
// decoded document.
func (s *Store) makeDocumentVisitor(rows *sql.Rows, queryErr error) func(func(string, semantic.DocumentData)) error {
	return func(f func(string, semantic.DocumentData)) (err error) {
		if queryErr != nil {
			return queryErr
		}
		defer func() { err = basestore.CloseRows(rows, err) }()

		var rawData []byte
		for rows.Next() {
			var path string
			if err := rows.Scan(&path, &rawData); err != nil {
				return err
			}

			data, err := s.serializer.UnmarshalDocumentData(rawData)
			if err != nil {
				return err
			}

			f(path, data)
		}

		return nil
	}
}

// scanFirstDocumentData reads qualified document data values from the given row
// object and returns the first one. If no rows match the query, a false-valued
// flag is returned.
func (s *Store) scanFirstDocumentData(rows *sql.Rows, queryErr error) (_ QualifiedDocumentData, _ bool, err error) {
	if queryErr != nil {
		return QualifiedDocumentData{}, false, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	if rows.Next() {
		record, err := s.scanSingleDocumentDataObject(rows)
		if err != nil {
			return QualifiedDocumentData{}, false, err
		}

		return record, true, nil
	}

	return QualifiedDocumentData{}, false, nil
}

// scanSingleDocumentDataObject populates a qualified document data value from the
// given cursor.
func (s *Store) scanSingleDocumentDataObject(rows *sql.Rows) (QualifiedDocumentData, error) {
	var rawData []byte
	var record QualifiedDocumentData
	if err := rows.Scan(&record.UploadID, &record.Path, &rawData); err != nil {
		return QualifiedDocumentData{}, err
	}

	data, err := s.serializer.UnmarshalDocumentData(rawData)
	if err != nil {
		return QualifiedDocumentData{}, err
	}
	record.Document = data

	return record, nil
}

// makeResultChunkVisitor returns a function that accepts a mapping function, reads
// result chunk values from the given row object and calls the mapping function on
// each decoded result set.
func (s *Store) makeResultChunkVisitor(rows *sql.Rows, queryErr error) func(func(int, semantic.ResultChunkData)) error {
	return func(f func(int, semantic.ResultChunkData)) (err error) {
		if queryErr != nil {
			return queryErr
		}
		defer func() { err = basestore.CloseRows(rows, err) }()

		var rawData []byte
		for rows.Next() {
			var index int
			if err := rows.Scan(&index, &rawData); err != nil {
				return err
			}

			data, err := s.serializer.UnmarshalResultChunkData(rawData)
			if err != nil {
				return err
			}

			f(index, data)
		}

		return nil
	}
}

type QualifiedMonikerLocations struct {
	DumpID int
	semantic.MonikerLocations
}

// scanQualifiedMonikerLocations reads moniker locations values from the given row object.
func (s *Store) scanQualifiedMonikerLocations(rows *sql.Rows, queryErr error) (_ []QualifiedMonikerLocations, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	var values []QualifiedMonikerLocations
	for rows.Next() {
		record, err := s.scanSingleQualifiedMonikerLocationsObject(rows)
		if err != nil {
			return nil, err
		}

		values = append(values, record)
	}

	return values, nil
}

// scanSingleQualifiedMonikerLocationsObject populates a qualified moniker locations value
// from the given cursor.
func (s *Store) scanSingleQualifiedMonikerLocationsObject(rows *sql.Rows) (QualifiedMonikerLocations, error) {
	var rawData []byte
	var record QualifiedMonikerLocations
	if err := rows.Scan(&record.DumpID, &record.Scheme, &record.Identifier, &rawData); err != nil {
		return QualifiedMonikerLocations{}, err
	}

	data, err := s.serializer.UnmarshalLocations(rawData)
	if err != nil {
		return QualifiedMonikerLocations{}, err
	}
	record.Locations = data

	return record, nil
}
