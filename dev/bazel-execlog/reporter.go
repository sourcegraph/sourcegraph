package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/go-cmp/cmp"
)

type DiffReporter struct {
	path  cmp.Path
	lines []string
}

func (r *DiffReporter) PushStep(ps cmp.PathStep) {
	r.path = append(r.path, ps)

	vx, vy := ps.Values()

	if p, ok := ps.(cmp.StructField); ok && p.Type().Kind() == reflect.Slice && r.path.Index(-2).Type().Field(p.Index()).IsExported() {
		typName := p.Type().Elem().Name()
		if p.Type().Elem().Kind() == reflect.Pointer {
			typName = p.Type().Elem().Elem().Name()
		}
		str := fmt.Sprintf("%s%s: []%s{", strings.Repeat("  ", len(r.path)), p.Name(), typName)

		if vx.Len() == 0 {
			str += "}"
		}

		r.lines = append(r.lines, str)
	}
	// For structs that were newly added/removed in whole (vx or vy is nil), we don't do anything as render handles that with diff markers.
	// Matching for close brace in PopStop.
	if p, ok := ps.(cmp.StructField); ok && p.Type().Kind() == reflect.Pointer && p.Type().Elem().Kind() == reflect.Struct && (!vx.IsNil() && !vy.IsNil()) && r.path.Index(-2).Type().Field(p.Index()).IsExported() {
		r.lines = append(r.lines, fmt.Sprintf("%s%s: %s{", strings.Repeat("  ", len(r.path)), p.Name(), p.Type().Elem().Name()))
	}
	// Not 100% sure anymore why IsValid is needed.
	// Matching for close brace in PopStop.
	if p, ok := ps.(cmp.SliceIndex); ok && p.Type().Kind() == reflect.Pointer && p.Type().Elem().Kind() == reflect.Struct && vx.IsValid() && vy.IsValid() {
		r.lines = append(r.lines, fmt.Sprintf("%s%s{", strings.Repeat("  ", len(r.path)), p.Type().Elem().Name()))
	}
}

func (r *DiffReporter) Report(rs cmp.Result) {
	if !rs.Equal() {
		vx, vy := r.path.Last().Values()
		switch msg := r.path.Last().(type) {
		case cmp.StructField:
			r.lines = append(r.lines, fmt.Sprintf("-%s%s: %s", strings.Repeat(" ", len(r.path)*2-1), msg.Name(), r.render(vx, "-", 0)))
			r.lines = append(r.lines, fmt.Sprintf("+%s%s: %s", strings.Repeat(" ", len(r.path)*2-1), msg.Name(), r.render(vy, "+", 0)))
		case cmp.SliceIndex:
			if vx.IsValid() {
				r.lines = append(r.lines, fmt.Sprintf("-%s%s", strings.Repeat(" ", len(r.path)*2-1), r.render(vx, "-", 0)))
			}
			if vy.IsValid() {
				r.lines = append(r.lines, fmt.Sprintf("+%s%s", strings.Repeat(" ", len(r.path)*2-1), r.render(vy, "+", 0)))
			}
		default:
			r.lines = append(r.lines, fmt.Sprintf("-%s%s: %s", strings.Repeat(" ", len(r.path)*2-1), msg.String(), r.render(vx, "-", 0)))
			r.lines = append(r.lines, fmt.Sprintf("+%s%s: %s", strings.Repeat(" ", len(r.path)*2-1), msg.String(), r.render(vy, "+", 0)))
		}
	} else if !rs.ByIgnore() {
		// no change and not an ignored (aka unexported) field, render straight without diff markers
		vx, _ := r.path.Last().Values()
		switch msg := r.path.Last().(type) {
		case cmp.StructField:
			// handled in PushStep above
			if msg.Type().Kind() == reflect.Slice {
				break
			}
			r.lines = append(r.lines, fmt.Sprintf("%s%s: %s", strings.Repeat("  ", len(r.path)), msg.Name(), r.render(vx, " ", 0)))
		case cmp.SliceIndex:
			r.lines = append(r.lines, fmt.Sprintf("%s%s", strings.Repeat("  ", len(r.path)), r.render(vx, " ", 0)))
		default:
			r.lines = append(r.lines, fmt.Sprintf("%s%s: %s", strings.Repeat("  ", len(r.path)), msg.String(), r.render(vx, " ", 0)))
		}
	}
}

func (r *DiffReporter) PopStep() {
	// Here we need to handle the closing brackets for slices and structs which were only partially changed. Any full changes (newly added/removed) are handled by render below

	vx, vy := r.path.Last().Values()

	// For empty slices, we don't need to do anything as we've already closed the slice above in PushStep.
	// Matching for open brace in PushStep.
	if p, ok := r.path.Last().(cmp.StructField); ok && p.Type().Kind() == reflect.Slice && vx.Len() != 0 && r.path.Index(-2).Type().Field(p.Index()).IsExported() {
		r.lines = append(r.lines, strings.Repeat("  ", len(r.path))+"}")
	}
	// For structs that were newly added/removed in whole (vx or vy is nil), we don't do anything as render handles that with diff markers.
	// Matching for open brace in PushStep.
	if p, ok := r.path.Last().(cmp.StructField); ok && p.Type().Kind() == reflect.Pointer && p.Type().Elem().Kind() == reflect.Struct && (!vx.IsNil() && !vy.IsNil()) && r.path.Index(-2).Type().Field(p.Index()).IsExported() {
		r.lines = append(r.lines, strings.Repeat("  ", len(r.path))+"}")
	}
	// Not 100% sure anymore why IsValid is needed.
	// Matching for open brace in PushStep.
	if p, ok := r.path.Last().(cmp.SliceIndex); ok && p.Type().Kind() == reflect.Pointer && p.Type().Elem().Kind() == reflect.Struct && vx.IsValid() && vy.IsValid() {
		r.lines = append(r.lines, strings.Repeat("  ", len(r.path))+"}")
	}

	r.path = r.path[:len(r.path)-1]
}

func (r *DiffReporter) String() string {
	return strings.Join(r.lines, "\n")
}

func (r *DiffReporter) render(val reflect.Value, diffMarker string, depth int) string {
	if val.Kind() == reflect.String {
		return "\"" + val.String() + "\""
	}

	if val.Kind() == reflect.Pointer && val.Elem().Kind() == reflect.Struct {
		return r.render(val.Elem(), diffMarker, depth)
	}

	// go-cmp will call Report above for the smallest changes between two values. If these happen to be basic concrete types e.g. string/number/bool, then the diff will be a single line of text
	// handled by a simple fmt.Sprint (or the extra quoting + val.String() above for string types). This works for struct fields or slice elements.
	// When an entire struct or slice differs (e.g. it doesn't exist on one side of the diff), then go-cmp will not call Report for each field/element, so we need to handle that here.

	if val.Kind() == reflect.Struct {
		var b strings.Builder
		b.WriteString(val.Type().Name())
		b.WriteString("{\n")
		for i := 0; i < val.NumField(); i++ {
			if !val.Type().Field(i).IsExported() {
				continue
			}
			f := val.Field(i)
			b.WriteByte(diffMarker[0])
			b.WriteString(strings.Repeat(" ", (len(r.path)+1+depth)*2-1))
			b.WriteString(val.Type().Field(i).Name)
			b.WriteString(": ")
			b.WriteString(r.render(f, diffMarker, depth+1))
			b.WriteRune('\n')
		}
		b.WriteByte(diffMarker[0])
		b.WriteString(strings.Repeat(" ", (len(r.path)+depth)*2-1))
		b.WriteRune('}')
		return b.String()
	}

	if val.Kind() == reflect.Slice {
		var b strings.Builder
		b.WriteString("[]")
		if val.Type().Elem().Kind() == reflect.Pointer {
			b.WriteString(val.Type().Elem().Elem().Name())
		} else {
			b.WriteString(val.Type().Elem().Name())
		}
		b.WriteRune('{')
		for i := 0; i < val.Len(); i++ {
			b.WriteRune('\n')
			b.WriteByte(diffMarker[0])
			b.WriteString(strings.Repeat(" ", (len(r.path)+1+depth)*2-1))
			b.WriteString(r.render(val.Index(i), diffMarker, depth+1))
		}
		if val.Len() > 0 {
			b.WriteRune('\n')
			b.WriteByte(diffMarker[0])
			b.WriteString(strings.Repeat(" ", (len(r.path)+depth)*2-1))
		}
		b.WriteRune('}')
		return b.String()
	}

	if val.Kind() == reflect.Interface {
		return r.render(val.Elem(), diffMarker, depth)
	}

	return fmt.Sprint(val)
}
