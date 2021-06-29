package cmdtest

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

// Error expects cmd to fail with an error that unwraps to want. Error returns
// the command output but does not validate the output.
func Error(t *testing.T, cmd *cobra.Command, args []string, want error) string {
	cmd.SetArgs(args)

	var out bytes.Buffer
	cmd.SetOutput(&out)

	err := cmd.Execute()
	if !errors.Is(err, want) {
		t.Fatalf("Command should fail with %q; got %q", want, err)
	}

	return out.String()
}

// Output expects cmd to output fmt.Sprint(want) and returns the actual output.
func Output(t *testing.T, cmd *cobra.Command, args []string, want interface{}) string {
	cmd.SetArgs(args)

	var out bytes.Buffer
	cmd.SetOutput(&out)
	cmd.Execute()

	result := out.String()
	wantStr := fmt.Sprint(want)
	if result != wantStr {
		t.Fatalf("Command has wrong output.\n\nwant:\n%v\n\ngot:\n%v\n", wantStr, result)
	}

	return result
}

// TableOutput expects cmd to output want as a table and returns the actual
// output. If colorize is non-nil, it is used to colorize the expected output
// before comparing to the actual output.
func TableOutput(t *testing.T, cmd *cobra.Command, args []string, want [][]string, colorize func(interface{}) aurora.Value) string {
	cmd.SetArgs(args)

	var out bytes.Buffer
	cmd.SetOutput(&out)
	cmd.Execute()

	var builder strings.Builder
	tabw := tabwriter.NewWriter(&builder, 0, 2, 1, ' ', 0)
	for _, row := range want {
		col := strings.Join(row, "\t")
		fmt.Fprintln(tabw, col)
	}
	if err := tabw.Flush(); err != nil {
		panic(fmt.Errorf("flush tabwriter: %w", err))
	}

	result := out.String()
	wantStr := fmt.Sprint(builder.String())

	if colorize != nil {
		wantStr = colorize(wantStr).String()
	}

	if result != wantStr {
		t.Fatalf("Command has wrong output.\n\nwant:\n%v\n\ngot:\n%v\n", wantStr, result)
	}

	return builder.String()
}
