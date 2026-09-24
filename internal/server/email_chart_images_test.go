package server

import (
	"encoding/base64"
	"fmt"
	"image/png"
	"math"
	"strings"
	"testing"
	"well-ambient/internal/config"

	nethtml "golang.org/x/net/html"
)

func TestEmailPNGRingProportionsAndThemeContrast(t *testing.T) {
	canvas := emailRingImage([]float64{0.25}, emailImagePalette, 33)
	filled, visible := 0, 0
	for y := 0; y < canvas.Bounds().Dy(); y++ {
		for x := 0; x < canvas.Bounds().Dx(); x++ {
			alpha := canvas.NRGBAAt(x, y).A
			if alpha > 0 {
				visible++
			}
			if alpha == 255 {
				filled++
			}
		}
	}
	if math.Abs(float64(filled)/float64(visible)-0.25) > 0.005 {
		t.Fatal("ring pixels no longer represent the input proportion")
	}
	luminance := func(r, g, b uint8) float64 {
		channel := func(v uint8) float64 {
			x := float64(v) / 255
			if x <= 0.04045 {
				return x / 12.92
			}
			return math.Pow((x+0.055)/1.055, 2.4)
		}
		return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
	}
	for _, paint := range emailImagePalette {
		ink := luminance(paint.R, paint.G, paint.B)
		for _, surface := range []float64{luminance(248, 250, 252), luminance(32, 44, 62)} {
			ratio := (math.Max(ink, surface) + 0.05) / (math.Min(ink, surface) + 0.05)
			if ratio < 3 {
				t.Errorf("bitmap series color loses contrast on an email theme: %.2f", ratio)
			}
		}
	}
}

func TestEmailPNGKeepsAllStatusAndTrendFactsOutsideImages(t *testing.T) {
	counts := map[string]int{}
	for i := 1; i <= 10; i++ {
		counts[fmt.Sprintf("status-%02d", i)] = i
	}
	_, legend := renderPieChartImage(counts)
	for label, count := range counts {
		if !strings.Contains(string(legend), fmt.Sprintf("%s %d", label, count)) {
			t.Errorf("status %s lost its HTML count", label)
		}
	}
	if strings.Contains(string(legend), `class="chart-series-`) {
		t.Fatal("dark SVG rules must not change swatches independently of PNG pixels")
	}
	trend, _ := renderCurveChartImage([]int{4, 6, 5, 8, 7, 5, 3}, []string{"01/08", "01/09", "01/10", "01/11", "01/12", "01/13", "01/14"})
	for _, fact := range []string{"01/08", "01/14", `>8</td>`} {
		if !strings.Contains(string(trend), fact) {
			t.Errorf("trend lost HTML fact %q", fact)
		}
	}
}

func TestEmailChartsUseDecodableInlinePNG(t *testing.T) {
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		t.Run(style, func(t *testing.T) {
			report, err := emailTemplateDemo(config.EmailTemplate{Style: style})
			if err != nil {
				t.Fatal(err)
			}
			doc, err := nethtml.Parse(strings.NewReader(report.HTML))
			if err != nil {
				t.Fatal(err)
			}
			images := 0
			var visit func(*nethtml.Node)
			visit = func(node *nethtml.Node) {
				if node.Type == nethtml.ElementNode && node.Data == "svg" {
					t.Error("email still depends on inline SVG")
				}
				if node.Type == nethtml.ElementNode && node.Data == "img" {
					images++
					attrs := map[string]string{}
					for _, attr := range node.Attr {
						attrs[attr.Key] = attr.Val
					}
					if !strings.HasPrefix(attrs["src"], "data:image/png;base64,") || attrs["alt"] == "" || attrs["width"] == "" || attrs["height"] == "" {
						t.Error("chart image needs a local PNG, alternative text and explicit dimensions")
						return
					}
					data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(attrs["src"], "data:image/png;base64,"))
					if err != nil {
						t.Fatal(err)
					}
					image, err := png.Decode(strings.NewReader(string(data)))
					if err != nil {
						t.Fatal(err)
					}
					if image.Bounds().Dx() < 2 || image.Bounds().Dy() < 2 {
						t.Fatal("chart was replaced by a tracking pixel")
					}
					visible := false
					for y := 0; y < image.Bounds().Dy() && !visible; y++ {
						for x := 0; x < image.Bounds().Dx(); x++ {
							_, _, _, alpha := image.At(x, y).RGBA()
							if alpha > 0 {
								visible = true
								break
							}
						}
					}
					if !visible {
						t.Fatal("chart image is entirely transparent")
					}
				}
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					visit(child)
				}
			}
			visit(doc)
			if images < 5 {
				t.Fatalf("only %d images; top and lower charts must all use mail-compatible images", images)
			}
		})
	}
}
