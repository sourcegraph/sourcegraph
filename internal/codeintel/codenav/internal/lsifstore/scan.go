package lsifstore

import (
	"bytes"
	"database/sql"
	"fmt"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/ranges"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

type qualifiedDocumentData struct {
	UploadID int
	Path     string
	LSIFData *precise.DocumentData
	SCIPData *scip.Document
}

func (s *store) scanDocumentData(rows *sql.Rows, queryErr error) (_ []qualifiedDocumentData, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	var values []qualifiedDocumentData
	for rows.Next() {
		record, err := s.scanSingleDocumentDataObject(rows)
		if err != nil {
			return nil, err
		}

		values = append(values, record)
	}

	return values, nil
}

func (s *store) scanFirstDocumentData(rows *sql.Rows, queryErr error) (_ qualifiedDocumentData, _ bool, err error) {
	if queryErr != nil {
		return qualifiedDocumentData{}, false, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	if rows.Next() {
		record, err := s.scanSingleDocumentDataObject(rows)
		if err != nil {
			return qualifiedDocumentData{}, false, err
		}

		return record, true, nil
	}

	return qualifiedDocumentData{}, false, nil
}

func (s *store) scanSingleDocumentDataObject(rows *sql.Rows) (qualifiedDocumentData, error) {
	var uploadID int
	var path string
	var compressedSCIPPayload []byte

	if err := rows.Scan(&uploadID, &path, &compressedSCIPPayload); err != nil {
		return qualifiedDocumentData{}, err
	}

	scipPayload, err := shared.Decompressor.Decompress(bytes.NewReader(compressedSCIPPayload))
	if err != nil {
		return qualifiedDocumentData{}, err
	}

	var data scip.Document
	if err := proto.Unmarshal(scipPayload, &data); err != nil {
		return qualifiedDocumentData{}, err
	}

	qualifiedData := qualifiedDocumentData{
		UploadID: uploadID,
		Path:     path,
		SCIPData: &data,
	}
	return qualifiedData, nil
}

type qualifiedMonikerLocations struct {
	UploadID int
	precise.MonikerLocations
}

func (s *store) scanQualifiedMonikerLocations(rows *sql.Rows, queryErr error) (_ []qualifiedMonikerLocations, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	var values []qualifiedMonikerLocations
	for rows.Next() {
		record, err := s.scanSingleQualifiedMonikerLocationsObject(rows)
		if err != nil {
			return nil, err
		}

		values = append(values, record)
	}

	return values, nil
}

func (s *store) scanSingleQualifiedMonikerLocationsObject(rows *sql.Rows) (qualifiedMonikerLocations, error) {
	var uri string
	var scipPayload []byte
	var record qualifiedMonikerLocations

	if err := rows.Scan(&record.UploadID, &record.Scheme, &record.Identifier, &scipPayload, &uri); err != nil {
		return qualifiedMonikerLocations{}, err
	}

	ranges, err := ranges.DecodeRanges(scipPayload)
	if err != nil {
		return qualifiedMonikerLocations{}, err
	}

	locations := make([]precise.LocationData, 0, len(ranges))
	for _, r := range ranges {
		locations = append(locations, precise.LocationData{
			URI:            uri,
			StartLine:      int(r.Start.Line),
			StartCharacter: int(r.Start.Character),
			EndLine:        int(r.End.Line),
			EndCharacter:   int(r.End.Character),
		})
	}

	record.Locations = locations
	return record, nil
}

//
//

func (s *store) scanDeduplicatedQualifiedMonikerLocations(rows *sql.Rows, queryErr error) (_ []qualifiedMonikerLocations, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	var values []qualifiedMonikerLocations
	for rows.Next() {
		record, err := s.scanSingleMinimalQualifiedMonikerLocationsObject(rows)
		if err != nil {
			return nil, err
		}

		if n := len(values) - 1; n >= 0 && values[n].UploadID == record.UploadID {
			values[n].Locations = append(values[n].Locations, record.Locations...)
		} else {
			values = append(values, record)
		}
	}
	for i := range values {
		values[i].Locations = deduplicate(values[i].Locations, locationDataKey)
	}

	return values, nil
}

func (s *store) scanSingleMinimalQualifiedMonikerLocationsObject(rows *sql.Rows) (qualifiedMonikerLocations, error) {
	var uri string
	var scipPayload []byte
	var record qualifiedMonikerLocations

	if err := rows.Scan(&record.UploadID, &scipPayload, &uri); err != nil {
		return qualifiedMonikerLocations{}, err
	}

	ranges, err := ranges.DecodeRanges(scipPayload)
	if err != nil {
		return qualifiedMonikerLocations{}, err
	}

	locations := make([]precise.LocationData, 0, len(ranges))
	for _, r := range ranges {
		locations = append(locations, precise.LocationData{
			URI:            uri,
			StartLine:      int(r.Start.Line),
			StartCharacter: int(r.Start.Character),
			EndLine:        int(r.End.Line),
			EndCharacter:   int(r.End.Character),
		})
	}

	record.Locations = locations
	return record, nil
}

func locationDataKey(v precise.LocationData) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d", v.URI, v.StartLine, v.StartCharacter, v.EndLine, v.EndCharacter)
}
