package symbols

import (
	"fmt"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

type ExplodedSymbol struct {
	Scheme                string
	PackageManager        string
	PackageName           string
	PackageVersion        string
	DescriptorNamespace   string
	DescriptorSuffix      string
	FuzzyDescriptorSuffix string // N.B.
}

func (s *ExplodedSymbol) String() string {
	return s.Symbol()
}

func (s *ExplodedSymbol) Symbol() string {
	return fmt.Sprintf(
		"%s %s%s",
		SchemeAndPackageOnlyFormatter.FormatSymbol(&scip.Symbol{
			Scheme: s.Scheme,
			Package: &scip.Package{
				Manager: s.PackageManager,
				Name:    s.PackageName,
				Version: s.PackageVersion,
			},
		}),
		s.DescriptorNamespace,
		s.DescriptorSuffix,
	)
}

func NewExplodedSymbol(symbol string) (*ExplodedSymbol, error) {
	parsedSymbol, err := scip.ParseSymbol(symbol)
	if err != nil {
		return nil, err
	}
	namespaceDescriptors, nonNamespaceDescriptors := splitNamespaces(parsedSymbol)

	namespace := ""
	if namespaceDescriptors != nil {
		namespace = scip.DescriptorOnlyFormatter.FormatSymbol(namespaceDescriptors)
	}

	return &ExplodedSymbol{
		Scheme:                parsedSymbol.Scheme,
		PackageManager:        parsedSymbol.Package.Manager,
		PackageName:           parsedSymbol.Package.Name,
		PackageVersion:        parsedSymbol.Package.Version,
		DescriptorNamespace:   namespace,
		DescriptorSuffix:      scip.DescriptorOnlyFormatter.FormatSymbol(nonNamespaceDescriptors),
		FuzzyDescriptorSuffix: ReducedDescriptorOnlyFormatter.FormatSymbol(nonNamespaceDescriptors),
	}, nil
}

func splitNamespaces(p *scip.Symbol) (*scip.Symbol, *scip.Symbol) {
	for i, descriptor := range p.Descriptors {
		if descriptor.Suffix != scip.Descriptor_Namespace {
			if i == 0 {
				// no namespaces
				return nil, p
			}

			prefix := &scip.Symbol{Scheme: p.Scheme, Package: p.Package, Descriptors: p.Descriptors[:i]}
			suffix := &scip.Symbol{Scheme: p.Scheme, Package: p.Package, Descriptors: p.Descriptors[i:]}
			return prefix, suffix

		}
	}

	// no non-namespaces
	return nil, p
}

var SchemeAndPackageOnlyFormatter = scip.SymbolFormatter{
	OnError:               func(err error) error { return err },
	IncludeScheme:         func(scheme string) bool { return true },
	IncludePackageManager: func(_ string) bool { return true },
	IncludePackageName:    func(_ string) bool { return true },
	IncludePackageVersion: func(_ string) bool { return true },
	IncludeDescriptor:     func(_ string) bool { return false },
	IncludeRawDescriptor:  func(_ *scip.Descriptor) bool { return false },
	IncludeDisambiguator:  func(_ string) bool { return false },
}

// ReducedDescriptorOnlyFormatter formats a reduced descriptor omitting suffixes outside
// of an explicit allowlist (currently including namespace, type, term, and method) as
// well as method disambiguators.
//
// This formatter is meant to produce a "good enough" representation of the symbol that
// can used to search for or match against a list of compiler-accurate SCIP symbols. The
// suffixes in the allowlist are chosen as they are, in most cases, producible given only
// a syntax tree.
var ReducedDescriptorOnlyFormatter = scip.SymbolFormatter{
	OnError:               func(err error) error { return err },
	IncludeScheme:         func(scheme string) bool { return scheme == "local" },
	IncludePackageManager: func(_ string) bool { return false },
	IncludePackageName:    func(_ string) bool { return false },
	IncludePackageVersion: func(_ string) bool { return false },
	IncludeDescriptor:     func(_ string) bool { return true },
	IncludeRawDescriptor:  includeReducedRawDescriptor,
	IncludeDisambiguator:  func(_ string) bool { return false },
}

var reducedSuffixes = []scip.Descriptor_Suffix{
	scip.Descriptor_Namespace,
	scip.Descriptor_Type,
	scip.Descriptor_Term,
	scip.Descriptor_Method,
}

func includeReducedRawDescriptor(descriptor *scip.Descriptor) bool {
	for _, suffix := range reducedSuffixes {
		if suffix == descriptor.Suffix {
			return true
		}
	}

	return false
}
