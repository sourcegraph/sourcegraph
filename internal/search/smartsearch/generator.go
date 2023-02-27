package smartsearch

import (
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"gonum.org/v1/gonum/stat/combin"
)

// next is the continuation for the query generator.
type next func() (*autoQuery, next)

type cg = combin.CombinationGenerator

type PHASE int

const (
	ONE PHASE = iota + 1
	TWO
	THREE
)

// NewComboGenerator returns a generator for queries produced by a combination
// of rules on a seed query. The generator has a strategy over two kinds of rule
// sets: narrowing and widening rules. You can read more below, but if you don't
// care about this and just want to apply rules sequentially, simply pass in
// only `widen` rules and pass in an empty `narrow` rule set. This will mean
// your queries are just generated by successively applying rules in order of
// the `widen` rule set. To get more sophisticated generation behavior, read on.
//
// This generator understands two kinds of rules:
//
// - narrowing rules (roughly, rules that we expect make a query more specific, and reduces the result set size)
// - widening rules (roughly, rules that we expect make a query more general, and increases the result set size).
//
// A concrete example of a narrowing rule might be: `go parse` -> `lang:go
// parse`. This since we restrict the subset of files to search for `parse` to
// Go files only.
//
// A concrete example of a widening rule might be: `a b` -> `a OR b`. This since
// the `OR` expression is more general and will typically find more results than
// the string `a b`.
//
// The way the generator applies narrowing and widening rules has three phases,
// executed in order. The phases work like this:
//
// PHASE ONE: The generator strategy tries to first apply _all narrowing_ rules,
// and then successively reduces the number of rules that it attempts to apply
// by one. This strategy is useful when we try the most aggressive
// interpretation of a query subject to rules first, and gradually loosen the
// number of rules and interpretation. Roughly, PHASE ONE can be thought of as
// trying to maximize applying "for all" rules on the narrow rule set.
//
// PHASE TWO: The generator performs PHASE ONE generation, generating
// combinations of narrow rules, and then additionally _adds_ the first widening
// rule to each narrowing combination. It continues iterating along the list of
// widening rules, appending them to each narrowing combination until the
// iteration of widening rules is exhausted. Roughly, PHASE TWO can be thought
// of as trying to maximize applying "for all" rules in the narrow rule set
// while widening them by applying, in order, "there exists" rules in the widen
// rule set.
//
// PHASE THREE: The generator only applies widening rules in order without any
// narrowing rules. Roughly, PHASE THREE can be thought of as an ordered "there
// exists" application over widen rules.
//
// To avoid spending time on generator invalid combinations, the generator
// prunes the initial rule set to only those rules that do successively apply
// individually to the seed query.
func NewGenerator(seed query.Basic, narrow, widen []rule) next {
	narrow = pruneRules(seed, narrow)
	widen = pruneRules(seed, widen)
	num := len(narrow)

	// the iterator state `n` stores:
	// - phase, the current generation phase based on progress
	// - k, the size of the selection in the narrow set to apply
	// - cg, an iterator producing the next sequence of rules for the current value of `k`.
	// - w, the index of the widen rule to apply (-1 if empty)
	var n func(phase PHASE, k int, c *cg, w int) next
	n = func(phase PHASE, k int, c *cg, w int) next {
		var transform []transform
		var descriptions []string
		var generated *query.Basic

		narrowing_exhausted := k == 0
		widening_active := w != -1
		widening_exhausted := widening_active && w == len(widen)

		switch phase {
		case THREE:
			if widening_exhausted {
				// Base case: we exhausted the set of narrow
				// rules (if any) and we've attempted every
				// widen rule with the sets of narrow rules.
				return nil
			}

			transform = append(transform, widen[w].transform...)
			descriptions = append(descriptions, widen[w].description)
			w += 1 // advance to next widening rule.

		case TWO:
			if widening_exhausted {
				// Start phase THREE: apply only widening rules.
				return n(THREE, 0, nil, 0)
			}

			if narrowing_exhausted && !widening_exhausted {
				// Continue widening: We've exhausted the sets of narrow
				// rules for the current widen rule, but we're not done
				// yet: there are still more widen rules to try. So
				// increment w by 1.
				c = combin.NewCombinationGenerator(num, num)
				w += 1 // advance to next widening rule.
				return n(phase, num, c, w)
			}

			if !c.Next() {
				// Reduce narrow set size.
				k -= 1
				c = combin.NewCombinationGenerator(num, k)
				return n(phase, k, c, w)
			}

			for _, idx := range c.Combination(nil) {
				transform = append(transform, narrow[idx].transform...)
				descriptions = append(descriptions, narrow[idx].description)
			}

			// Compose narrow rules with a widen rule.
			transform = append(transform, widen[w].transform...)
			descriptions = append(descriptions, widen[w].description)

		case ONE:
			if narrowing_exhausted && !widening_active {
				// Start phase TWO: apply widening with
				// narrowing rules. We've exhausted the sets of
				// narrow rules, but have not attempted to
				// compose them with any widen rules. Compose
				// them with widen rules by initializing w to 0.
				cg := combin.NewCombinationGenerator(num, num)
				return n(TWO, num, cg, 0)
			}

			if !c.Next() {
				// Reduce narrow set size.
				k -= 1
				c = combin.NewCombinationGenerator(num, k)
				return n(phase, k, c, w)
			}

			for _, idx := range c.Combination(nil) {
				transform = append(transform, narrow[idx].transform...)
				descriptions = append(descriptions, narrow[idx].description)
			}
		}

		generated = applyTransformation(seed, transform)
		if generated == nil {
			// Rule does not apply, go to next rule.
			return n(phase, k, c, w)
		}

		q := autoQuery{
			description: strings.Join(descriptions, " ⚬ "),
			query:       *generated,
		}

		return func() (*autoQuery, next) {
			return &q, n(phase, k, c, w)
		}
	}

	if len(narrow) == 0 {
		return n(THREE, 0, nil, 0)
	}

	cg := combin.NewCombinationGenerator(num, num)
	return n(ONE, num, cg, -1)
}

// pruneRules produces a minimum set of rules that apply successfully on the seed query.
func pruneRules(seed query.Basic, rules []rule) []rule {
	types, _ := seed.IncludeExcludeValues(query.FieldType)
	for _, t := range types {
		// Running additional diff searches is expensive, we clamp this
		// until things improve.
		if t == "diff" {
			return []rule{}
		}
	}

	applies := make([]rule, 0, len(rules))
	for _, r := range rules {
		g := applyTransformation(seed, r.transform)
		if g == nil {
			continue
		}
		applies = append(applies, r)
	}
	return applies
}

// applyTransformation applies a transformation on `b`. If any function does not apply, it returns nil.
func applyTransformation(b query.Basic, transform []transform) *query.Basic {
	for _, apply := range transform {
		res := apply(b)
		if res == nil {
			return nil
		}
		b = *res
	}
	return &b
}
