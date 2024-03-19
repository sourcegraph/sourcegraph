package main

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

var funkyLogoCommand = &cli.Command{
	Name:        "logo",
	ArgsUsage:   "[classic]",
	Usage:       "Print the sg logo",
	Description: "By default, prints the sg logo in different colors. When the 'classic' argument is passed it prints the classic logo.",
	Category:    category.Util,
	Action:      logoExec,
}

var styleOrange = output.Fg256Color(202)

func printLogo(out io.Writer) {
	fmt.Fprintf(out, "%s", output.StyleLogo)
	fmt.Fprintln(out, `          _____                    _____`)
	fmt.Fprintln(out, `         /\    \                  /\    \`)
	fmt.Fprintf(out, `        /%s::%s\    \                /%s::%s\    \`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `       /%s::::%s\    \              /%s::::%s\    \`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `      /%s::::::%s\    \            /%s::::::%s\    \`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `     /%s:::%s/\%s:::%s\    \          /%s:::%s/\%s:::%s\    \`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `    /%s:::%s/__\%s:::%s\    \        /%s:::%s/  \%s:::%s\    \`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `    \%s:::%s\   \%s:::%s\    \      /%s:::%s/    \%s:::%s\    \`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `  ___\%s:::%s\   \%s:::%s\    \    /%s:::%s/    / \%s:::%s\    \`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, ` /\   \%s:::%s\   \%s:::%s\    \  /%s:::%s/    /   \%s:::%s\ ___\`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `/%s::%s\   \%s:::%s\   \%s:::%s\____\/%s:::%s/____/  ___\%s:::%s|    |`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `\%s:::%s\   \%s:::%s\   \%s::%s/    /\%s:::%s\    \ /\  /%s:::%s|____|`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, ` \%s:::%s\   \%s:::%s\   \/____/  \%s:::%s\    /%s::%s\ \%s::%s/    /`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `  \%s:::%s\   \%s:::%s\    \       \%s:::%s\   \%s:::%s\ \/____/`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `   \%s:::%s\   \%s:::%s\____\       \%s:::%s\   \%s:::%s\____\`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `    \%s:::%s\  /%s:::%s/    /        \%s:::%s\  /%s:::%s/    /`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `     \%s:::%s\/%s:::%s/    /          \%s:::%s\/%s:::%s/    /`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `      \%s::::::%s/    /            \%s::::::%s/    /`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `       \%s::::%s/    /              \%s::::%s/    /`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintf(out, `        \%s::%s/    /                \%s::%s/____/`, styleOrange, output.StyleLogo, styleOrange, output.StyleLogo)
	fmt.Fprintln(out)
	fmt.Fprintln(out, `         \/____/`)
	fmt.Fprintf(out, "%s", output.StyleReset)
}

func logoExec(ctx *cli.Context) error {
	args := ctx.Args().Slice()
	if len(args) == 1 && args[0] == "classic" {
		var logoOut bytes.Buffer
		printLogo(&logoOut)
		std.Out.Write(logoOut.String())
		return nil
	}

	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	randoColor := func() output.Style { return output.Fg256Color(r1.Intn(256)) }

	var (
		color1a = randoColor()
		color1b = randoColor()
		color1c = randoColor()
		color2  = output.StyleLogo
	)

	times := 20
	for i := range times {
		const linesPrinted = 23

		std.Out.Writef("%s", color2)
		std.Out.Write(`          _____                    _____`)
		std.Out.Write(`         /\    \                  /\    \`)
		std.Out.Writef(`        /%s::%s\    \                /%s::%s\    \`, color1a, color2, color1b, color2)
		std.Out.Writef(`       /%s::::%s\    \              /%s::::%s\    \`, color1a, color2, color1b, color2)
		std.Out.Writef(`      /%s::::::%s\    \            /%s::::::%s\    \`, color1a, color2, color1b, color2)
		std.Out.Writef(`     /%s:::%s/\%s:::%s\    \          /%s:::%s/\%s:::%s\    \`, color1a, color2, color1b, color2, color1c, color2, color1a, color2)
		std.Out.Writef(`    /%s:::%s/__\%s:::%s\    \        /%s:::%s/  \%s:::%s\    \`, color1a, color2, color1b, color2, color1c, color2, color1a, color2)
		std.Out.Writef(`    \%s:::%s\   \%s:::%s\    \      /%s:::%s/    \%s:::%s\    \`, color1a, color2, color1b, color2, color1c, color2, color1a, color2)
		std.Out.Writef(`  ___\%s:::%s\   \%s:::%s\    \    /%s:::%s/    / \%s:::%s\    \`, color1a, color2, color1b, color2, color1c, color2, color1a, color2)
		std.Out.Writef(` /\   \%s:::%s\   \%s:::%s\    \  /%s:::%s/    /   \%s:::%s\ ___\`, color1a, color2, color1b, color2, color1c, color2, color1a, color2)
		std.Out.Writef(`/%s::%s\   \%s:::%s\   \%s:::%s\____\/%s:::%s/____/  ___\%s:::%s|    |`, color1a, color2, color1b, color2, color1c, color2, color1a, color2, color1b, color2)
		std.Out.Writef(`\%s:::%s\   \%s:::%s\   \%s::%s/    /\%s:::%s\    \ /\  /%s:::%s|____|`, color1a, color2, color1b, color2, color1c, color2, color1a, color2, color1b, color2)
		std.Out.Writef(` \%s:::%s\   \%s:::%s\   \/____/  \%s:::%s\    /%s::%s\ \%s::%s/    /`, color1a, color2, color1b, color2, color1c, color2, color1a, color2, color1b, color2)
		std.Out.Writef(`  \%s:::%s\   \%s:::%s\    \       \%s:::%s\   \%s:::%s\ \/____/`, color1a, color2, color1b, color2, color1c, color2, color1a, color2)
		std.Out.Writef(`   \%s:::%s\   \%s:::%s\____\       \%s:::%s\   \%s:::%s\____\`, color1a, color2, color1b, color2, color1c, color2, color1a, color2)
		std.Out.Writef(`    \%s:::%s\  /%s:::%s/    /        \%s:::%s\  /%s:::%s/    /`, color1a, color2, color1b, color2, color1c, color2, color1a, color2)
		std.Out.Writef(`     \%s:::%s\/%s:::%s/    /          \%s:::%s\/%s:::%s/    /`, color1a, color2, color1b, color2, color1c, color2, color1a, color2)
		std.Out.Writef(`      \%s::::::%s/    /            \%s::::::%s/    /`, color1a, color2, color1b, color2)
		std.Out.Writef(`       \%s::::%s/    /              \%s::::%s/    /`, color1a, color2, color1b, color2)
		std.Out.Writef(`        \%s::%s/    /                \%s::%s/____/`, color1a, color2, color1b, color2)
		std.Out.Write(`         \/____/`)
		std.Out.Writef("%s", output.StyleReset)

		time.Sleep(200 * time.Millisecond)

		color1a, color1b, color1c, color2 = randoColor(), color1a, color1b, color1c

		if i != times-1 {
			std.Out.MoveUpLines(linesPrinted)
		}
	}

	return nil
}
