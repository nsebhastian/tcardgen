package cmd

import (
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nsebhastian/tcardgen/pkg/canvas"
	"github.com/nsebhastian/tcardgen/pkg/canvas/fontfamily"
	"github.com/nsebhastian/tcardgen/pkg/config"
	"github.com/nsebhastian/tcardgen/pkg/hugo"
)

const (
	defaultFontDir = "font"
	defaultOutDir  = "out"

	longDesc = `Generate TwitterCard(OGP) images for your Hugo posts.
Supported front-matters are title, author, categories, tags, and date.`
	example = `# Generate a image and output to the example directory.
tcardgen --fontDir=font --outDir=example --template=example/template.png example/blog-post.md

# Generate multiple images.
tcardgen --template=example/template.png example/*.md

# Genrate an image based on the drawing configuration.
tcardgen --config=config.yaml example/*.md`
)

var (
	// set values via build flags
	command string
	version string
	commit  string
)

type IOStreams struct {
	Out    io.Writer
	ErrOut io.Writer
}

type RootCommandOption struct {
	files   []string
	fontDir string
	outDir  string
	tplImg  string
	config  string
}

func NewRootCmd() *cobra.Command {
	opt := RootCommandOption{}
	cmd := &cobra.Command{
		Use:                   "tcardgen [-f <FONTDIR>] [-o <OUTDIR>] [-t <TEMPLATE>] [-c <CONFIG>] <FILE>...",
		Version:               version,
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
		SilenceErrors:         true,
		Short:                 "Generate TwitterCard(OGP) image for your Hugo posts.",
		Long:                  longDesc,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			streams := IOStreams{
				Out:    os.Stdout,
				ErrOut: os.Stderr,
			}
			if err := opt.Validate(cmd, args); err != nil {
				return err
			}
			return opt.Run(streams)
		},
	}
	cmd.Flags().StringVarP(&opt.fontDir, "fontDir", "f", defaultFontDir, "Set a font directory.")
	cmd.Flags().StringVarP(&opt.outDir, "outDir", "o", defaultOutDir, "Set an output directory.")
	cmd.Flags().StringVarP(&opt.tplImg, "template", "t", "", fmt.Sprintf("Set a template image file. (default %s)", config.DefaultTemplate))
	cmd.Flags().StringVarP(&opt.config, "config", "c", "", "Set a drawing configuration file.")
	return cmd
}

func (o *RootCommandOption) Validate(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("required argument <FILE> is not set")
	}
	o.files = args
	return nil
}

func (o *RootCommandOption) Run(streams IOStreams) error {
	ffa, err := fontfamily.LoadFromDir(o.fontDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(streams.Out, "Load fonts from %q\n", o.fontDir)

	cnf := &config.DrawingConfig{}
	if o.config != "" {
		cnf, err = config.LoadConfig(o.config)
		if err != nil {
			return err
		}
	}
	config.Defaulting(cnf, o.tplImg)

	tpl, err := canvas.LoadFromFile(cnf.Template)
	if err != nil {
		return err
	}
	fmt.Fprintf(streams.Out, "Load template from %q directory\n", cnf.Template)

	if _, err := os.Stat(o.outDir); os.IsNotExist(err) {
		err := os.Mkdir(o.outDir, 0755)
		if err != nil {
			return err
		}
	}

	var errCnt int
	for _, f := range o.files {
		base := filepath.Base(f)
		name := filepath.Base(f[:len(f)-len(base)])
		out := filepath.Join(o.outDir, fmt.Sprintf("%s.png", name))

		if err := generateTCard(f, out, tpl, ffa, cnf); err != nil {
			fmt.Fprintf(streams.ErrOut, "Failed to generate twitter card for %v: %v\n", out, err)
			errCnt++
			continue
		}
		fmt.Fprintf(streams.Out, "Success to generate twitter card into %v\n", out)
	}

	if errCnt != 0 {
		return fmt.Errorf("failed to generate %d twitter cards", errCnt)
	}
	return nil
}

func generateTCard(contentPath, outPath string, tpl image.Image, ffa *fontfamily.FontFamily, cnf *config.DrawingConfig) error {
	fm, err := hugo.ParseFrontMatter(contentPath)
	if err != nil {
		return err
	}

	c, err := canvas.CreateCanvasFromImage(tpl)
	if err != nil {
		return err
	}

	var tags []string
	for _, t := range fm.Tags {
		tags = append(tags, strings.Title(t))
	}

	if err := c.DrawTextAtPoint(
		fm.Title,
		*cnf.Title.Start,
		canvas.MaxWidth(cnf.Title.MaxWidth),
		canvas.LineSpacing(*cnf.Title.LineSpacing),
		canvas.FgHexColor(cnf.Title.FgHexColor),
		canvas.FontFaceFromFFA(ffa, cnf.Title.FontStyle, cnf.Title.FontSize),
	); err != nil {
		return err
	}
	if err := c.DrawTextAtPoint(
		"sebhastian.com",
		*cnf.Category.Start,
		canvas.FgHexColor(cnf.Category.FgHexColor),
		canvas.FontFaceFromFFA(ffa, cnf.Category.FontStyle, cnf.Category.FontSize),
	); err != nil {
		return err
	}
	if err := c.DrawTextAtPoint(
		"nsebhastian",
		*cnf.Info.Start,
		canvas.FgHexColor(cnf.Info.FgHexColor),
		canvas.FontFaceFromFFA(ffa, cnf.Info.FontStyle, cnf.Info.FontSize),
	); err != nil {
		return err
	}
	if err := c.DrawBoxTexts(
		tags,
		*cnf.Tags.Start,
		canvas.FgHexColor(cnf.Tags.FgHexColor),
		canvas.BgHexColor(cnf.Tags.BgHexColor),
		canvas.BoxPadding(*cnf.Tags.BoxPadding),
		canvas.BoxSpacing(*cnf.Tags.BoxSpacing),
		canvas.BoxAlign(cnf.Tags.BoxAlign),
		canvas.FontFaceFromFFA(ffa, cnf.Tags.FontStyle, cnf.Tags.FontSize),
	); err != nil {
		return err
	}

	return c.SaveAsPNG(outPath)
}
