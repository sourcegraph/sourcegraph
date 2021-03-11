package conversion

import "github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/lsif/protocol/reader"

type Element reader.Element
type Edge reader.Edge
type MetaData reader.MetaData
type PackageInformation reader.PackageInformation
type Diagnostic reader.Diagnostic

type Range struct {
	reader.Range
	DefinitionResultID int
	ReferenceResultID  int
	HoverResultID      int
}

func (r Range) SetDefinitionResultID(id int) Range {
	return Range{
		Range:              r.Range,
		DefinitionResultID: id,
		ReferenceResultID:  r.ReferenceResultID,
		HoverResultID:      r.HoverResultID,
	}
}

func (r Range) SetReferenceResultID(id int) Range {
	return Range{
		Range:              r.Range,
		DefinitionResultID: r.DefinitionResultID,
		ReferenceResultID:  id,
		HoverResultID:      r.HoverResultID,
	}
}

func (r Range) SetHoverResultID(id int) Range {
	return Range{
		Range:              r.Range,
		DefinitionResultID: r.DefinitionResultID,
		ReferenceResultID:  r.ReferenceResultID,
		HoverResultID:      id,
	}
}

type ResultSet struct {
	reader.ResultSet
	DefinitionResultID int
	ReferenceResultID  int
	HoverResultID      int
}

func (rs ResultSet) SetDefinitionResultID(id int) ResultSet {
	return ResultSet{
		ResultSet:          rs.ResultSet,
		DefinitionResultID: id,
		ReferenceResultID:  rs.ReferenceResultID,
		HoverResultID:      rs.HoverResultID,
	}
}

func (rs ResultSet) SetReferenceResultID(id int) ResultSet {
	return ResultSet{
		ResultSet:          rs.ResultSet,
		DefinitionResultID: rs.DefinitionResultID,
		ReferenceResultID:  id,
		HoverResultID:      rs.HoverResultID,
	}
}

func (rs ResultSet) SetHoverResultID(id int) ResultSet {
	return ResultSet{
		ResultSet:          rs.ResultSet,
		DefinitionResultID: rs.DefinitionResultID,
		ReferenceResultID:  rs.ReferenceResultID,
		HoverResultID:      id,
	}
}

type Moniker struct {
	reader.Moniker
	PackageInformationID int
}

func (m Moniker) SetPackageInformationID(id int) Moniker {
	return Moniker{
		Moniker:              m.Moniker,
		PackageInformationID: id,
	}
}
