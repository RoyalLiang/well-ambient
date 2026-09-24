package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"image"
	"image/color"
	"image/png"
	"math"
	"sort"
	"strconv"
	"strings"
)

const emailChartScale = 2

// The same pixels are used in previews and CID email attachments. These series
// colors are visible on both the light and dark email surfaces; CSS cannot
// recolor a PNG as it could an inline SVG.
var emailImagePalette = []color.NRGBA{
	{0, 143, 150, 255}, {77, 127, 220, 255}, {217, 119, 6, 255},
	{150, 100, 204, 255}, {5, 150, 105, 255}, {203, 86, 143, 255},
	{113, 129, 151, 255},
}

var emailImageTrack = color.NRGBA{148, 163, 184, 72}

func emailPNGTag(canvas *image.NRGBA, width, height int, alt, class string, fluid bool) template.HTML {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		return template.HTML(`<span class="email-text-muted">图表生成失败：` + html.EscapeString(alt) + `</span>`)
	}
	displayWidth := fmt.Sprint(width)
	style := fmt.Sprintf("display:inline-block;width:%dpx;height:%dpx;max-width:100%%;vertical-align:middle;border:0", width, height)
	if fluid {
		displayWidth = "100%"
		style = fmt.Sprintf("display:block;width:100%%;height:%dpx;border:0", height)
	}
	return template.HTML(fmt.Sprintf(`<img class="%s" src="data:image/png;base64,%s" width="%s" height="%d" alt="%s" style="%s">`,
		html.EscapeString(class), base64.StdEncoding.EncodeToString(encoded.Bytes()), displayWidth, height, html.EscapeString(alt), style))
}

func emailRingImage(shares []float64, colors []color.NRGBA, inner float64) *image.NRGBA {
	canvas := image.NewNRGBA(image.Rect(0, 0, 96*emailChartScale, 96*emailChartScale))
	for y := 0; y < canvas.Bounds().Dy(); y++ {
		for x := 0; x < canvas.Bounds().Dx(); x++ {
			dx := (float64(x)+0.5)/emailChartScale - 48
			dy := (float64(y)+0.5)/emailChartScale - 48
			radius := math.Hypot(dx, dy)
			if radius < inner || radius > 43 {
				continue
			}
			angle := math.Mod(math.Atan2(dy, dx)+math.Pi/2+2*math.Pi, 2*math.Pi) / (2 * math.Pi)
			paint := emailImageTrack
			start := 0.0
			for i, share := range shares {
				if angle >= start && angle < start+share {
					paint = colors[i%len(colors)]
					break
				}
				start += share
			}
			canvas.SetNRGBA(x, y, paint)
		}
	}
	return canvas
}

func renderDonutChartImage(resolved, total int) (template.HTML, string) {
	value, desc := "—", "昨日无更新事项"
	share := 0.0
	paint := emailImagePalette[0]
	if total > 0 {
		resolved = max(0, min(resolved, total))
		share = float64(resolved) / float64(total)
		rate := resolved * 100 / total
		value = fmt.Sprintf("%d%%", rate)
		desc = fmt.Sprintf("昨日更新中已解决 %d / %d 项", resolved, total)
		if rate >= 80 {
			paint = emailImagePalette[4]
		} else if rate < 50 {
			paint = emailImagePalette[2]
		}
	}
	img := emailPNGTag(emailRingImage([]float64{share}, []color.NRGBA{paint}, 33), 96, 96, desc, "email-resolution-image", false)
	return img + template.HTML(`<div class="email-h2" style="font-size:20px;font-weight:700;line-height:1.5;color:#0f172a">`+value+`</div>`), desc
}

func renderPieChartImage(counts map[string]int) (template.HTML, template.HTML) {
	type entry struct {
		label string
		count int
	}
	var entries []entry
	total := 0
	for label, count := range counts {
		if count > 0 {
			entries = append(entries, entry{label, count})
			total += count
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count == entries[j].count {
			return entries[i].label < entries[j].label
		}
		return entries[i].count > entries[j].count
	})
	var shares []float64
	var paints []color.NRGBA
	var legend strings.Builder
	legend.WriteString(`<div class="email-chart-legend" style="line-height:1.8;margin-top:4px;white-space:normal">`)
	for i, entry := range entries {
		shares = append(shares, float64(entry.count)/float64(total))
		paint := emailImagePalette[i%len(emailImagePalette)]
		switch strings.ToLower(strings.TrimSpace(entry.label)) {
		case "done", "closed", "resolved":
			paint = emailImagePalette[4]
		case "progress", "in progress":
			paint = emailImagePalette[1]
		case "review":
			paint = emailImagePalette[2]
		case "todo":
			paint = emailImagePalette[6]
		}
		paints = append(paints, paint)
		fmt.Fprintf(&legend, `<span class="chart-badge email-status-label" style="display:inline-block;padding:3px 7px;background-color:#ffffff;border:1px solid #cbd5e1;border-radius:6px;margin:3px 2px;font-size:12px;font-weight:600;color:#334155;white-space:normal;overflow-wrap:anywhere"><span class="email-chart-swatch" style="display:inline-block;width:10px;height:10px;border-radius:50%%;background-color:#%02x%02x%02x;margin-right:5px;box-shadow:0 0 0 1px rgba(148,163,184,0.5)"></span>%s %d</span>`,
			paint.R, paint.G, paint.B, html.EscapeString(entry.label), entry.count)
	}
	if total == 0 {
		legend.WriteString(`<span class="email-text-muted" style="font-size:12px;color:#536479">暂无状态分布</span>`)
	}
	legend.WriteString(`</div>`)
	img := emailPNGTag(emailRingImage(shares, paints, 27), 96, 96, fmt.Sprintf("昨日更新状态分布，共 %d 项；各状态数量见图例", total), "email-status-image", false)
	return img, template.HTML(legend.String())
}

func emailPaintDot(canvas *image.NRGBA, x, y, radius float64, paint color.NRGBA) {
	for py := int((y - radius) * emailChartScale); py <= int((y+radius)*emailChartScale); py++ {
		for px := int((x - radius) * emailChartScale); px <= int((x+radius)*emailChartScale); px++ {
			dx, dy := (float64(px)+0.5)/emailChartScale-x, (float64(py)+0.5)/emailChartScale-y
			if dx*dx+dy*dy <= radius*radius {
				canvas.SetNRGBA(px, py, paint)
			}
		}
	}
}

var emailDigitBitmaps = [10][5]uint8{
	{0b111, 0b101, 0b101, 0b101, 0b111}, // 0
	{0b010, 0b110, 0b010, 0b010, 0b111}, // 1
	{0b111, 0b001, 0b111, 0b100, 0b111}, // 2
	{0b111, 0b001, 0b111, 0b001, 0b111}, // 3
	{0b101, 0b101, 0b111, 0b001, 0b001}, // 4
	{0b111, 0b100, 0b111, 0b001, 0b111}, // 5
	{0b111, 0b100, 0b111, 0b101, 0b111}, // 6
	{0b111, 0b001, 0b001, 0b001, 0b001}, // 7
	{0b111, 0b101, 0b111, 0b101, 0b111}, // 8
	{0b111, 0b101, 0b111, 0b001, 0b111}, // 9
}

func emailPaintDigits(canvas *image.NRGBA, cx, cy float64, val int, paint color.NRGBA) {
	str := strconv.Itoa(val)
	totalW := float64(len(str)*4 - 1)
	startX := cx - totalW/2.0
	startY := cy - 8.5
	if cy < 20 {
		startY = cy + 4.5
	}
	bgHalo := color.NRGBA{255, 255, 255, 220}

	for charIdx, ch := range str {
		if ch < '0' || ch > '9' {
			continue
		}
		digit := ch - '0'
		bmp := emailDigitBitmaps[digit]
		digitX := startX + float64(charIdx*4)
		for row := 0; row < 5; row++ {
			bits := bmp[row]
			for col := 0; col < 3; col++ {
				if (bits & (1 << (2 - col))) != 0 {
					rx := digitX + float64(col)
					ry := startY + float64(row)
					for dy := -1; dy <= 1; dy++ {
						for dx := -1; dx <= 1; dx++ {
							hpx := int(rx*emailChartScale) + dx
							hpy := int(ry*emailChartScale) + dy
							if hpx >= 0 && hpx < canvas.Bounds().Dx() && hpy >= 0 && hpy < canvas.Bounds().Dy() {
								canvas.SetNRGBA(hpx, hpy, bgHalo)
							}
						}
					}
					px := int(rx * emailChartScale)
					py := int(ry * emailChartScale)
					for oy := 0; oy < emailChartScale; oy++ {
						for ox := 0; ox < emailChartScale; ox++ {
							if px+ox >= 0 && px+ox < canvas.Bounds().Dx() && py+oy >= 0 && py+oy < canvas.Bounds().Dy() {
								canvas.SetNRGBA(px+ox, py+oy, paint)
							}
						}
					}
				}
			}
		}
	}
}

func renderCurveChartImage(points []int, labels []string) (template.HTML, string) {
	if len(points) != 7 {
		return template.HTML(`<p class="email-text-muted" style="font-size:13px;color:#536479">暂无七日数据</p>`), "暂无完整七日数据"
	}
	maximum := 2
	for _, value := range points {
		if value < 0 {
			return "", "历史数据无效"
		}
		maximum = max(maximum, value)
	}
	// Three integer ticks, with the same scale used for the plotted samples.
	maximum = ((maximum + 1) / 2) * 2
	if len(labels) < 7 {
		labels = []string{"-7d", "-6d", "-5d", "-4d", "-3d", "-2d", "-1d"}
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, 160*emailChartScale, 90*emailChartScale))
	const baseline = 74.0
	xAt := func(i int) float64 { return 12 + float64(i)*136/6 }
	yAt := func(i int) float64 { return baseline - float64(points[i])*60/float64(maximum) }
	for _, y := range []int{14, 44, 74} {
		for x := 12 * emailChartScale; x < 149*emailChartScale; x++ {
			canvas.SetNRGBA(x, y*emailChartScale, emailImageTrack)
		}
	}
	for i := 0; i < 6; i++ {
		for px := int(xAt(i) * emailChartScale); px <= int(xAt(i+1)*emailChartScale); px++ {
			x := float64(px) / emailChartScale
			t := max(0, min(1, (x-xAt(i))/(xAt(i+1)-xAt(i))))
			// Bounded cubic interpolation preserves real samples without inventing
			// negative counts or overshooting a zero interval.
			y := yAt(i) + (yAt(i+1)-yAt(i))*(t*t*(3-2*t))
			for py := int(y * emailChartScale); py < int(baseline*emailChartScale); py++ {
				paint := emailImagePalette[0]
				paint.A = uint8(48 * (baseline - float64(py)/emailChartScale) / max(1, baseline-y))
				canvas.SetNRGBA(px, py, paint)
			}
			emailPaintDot(canvas, x, y, 1, emailImagePalette[0])
		}
	}
	for i := range points {
		emailPaintDot(canvas, xAt(i), yAt(i), 2.5, emailImagePalette[0])
		emailPaintDigits(canvas, xAt(i), yAt(i), points[i], emailImagePalette[0])
	}
	var data strings.Builder
	data.WriteString(`<table class="email-trend-values email-text-muted" role="presentation" width="160" cellpadding="0" cellspacing="0" style="width:100%;table-layout:fixed;text-align:center;font-size:10px;line-height:1.6;color:#536479;margin-top:4px"><tr>`)
	for _, label := range labels[:7] {
		fmt.Fprintf(&data, `<td style="padding:0;word-break:normal">%s</td>`, html.EscapeString(label))
	}
	data.WriteString(`</tr></table>`)
	desc := fmt.Sprintf("按最后更新时间 · 昨日 %d 项", points[6])
	img := emailPNGTag(canvas, 160, 90, fmt.Sprintf("近7日更新分布，纵轴 0 至 %d 项；各日相对天数与数量见下方", maximum), "email-trend-image", false)
	img = template.HTML(strings.Replace(string(img), "width:160px", "width:100%", 1))
	// HTML ticks remain themeable and readable even when the image is blocked.
	axis := fmt.Sprintf(`<table class="email-trend-axis email-text-muted" role="presentation" cellpadding="0" cellspacing="0" style="height:90px;font-size:10px;line-height:14px;text-align:right;color:#536479;white-space:nowrap;word-break:normal"><tr><td style="height:7px;padding:0"></td></tr><tr><td class="email-trend-tick" style="height:14px;padding:0 3px 0 0">%d</td></tr><tr><td style="height:16px;padding:0"></td></tr><tr><td class="email-trend-tick" style="height:14px;padding:0 3px 0 0">%d</td></tr><tr><td style="height:16px;padding:0"></td></tr><tr><td class="email-trend-tick" style="height:14px;padding:0 3px 0 0">0</td></tr><tr><td style="height:9px;padding:0"></td></tr></table>`, maximum, maximum/2)
	return template.HTML(`<table class="email-trend-plot" role="presentation" cellpadding="0" cellspacing="0" width="100%" style="width:100%;max-width:190px;table-layout:fixed;margin:0 auto;border-spacing:0"><tr><td width="30" valign="top" style="width:30px;padding:0">` + axis + `</td><td valign="top" style="padding:0;line-height:0">` + string(img) + `</td></tr><tr><td></td><td style="padding:0">` + data.String() + `</td></tr></table>`), desc
}

func emailProportionImage(count, total int, alt, class string) template.HTML {
	const width, height = 640, 10
	canvas := image.NewNRGBA(image.Rect(0, 0, width*emailChartScale, height*emailChartScale))
	share := 0.0
	if total > 0 {
		share = math.Max(0, math.Min(1, float64(count)/float64(total)))
	}
	for y := 0; y < canvas.Bounds().Dy(); y++ {
		for x := 0; x < canvas.Bounds().Dx(); x++ {
			px, py := (float64(x)+0.5)/emailChartScale, (float64(y)+0.5)/emailChartScale
			cx := math.Max(height/2.0, math.Min(width-height/2.0, px))
			if math.Hypot(px-cx, py-height/2.0) > height/2.0 {
				continue
			}
			paint := emailImageTrack
			if float64(x)+0.5 < share*float64(canvas.Bounds().Dx()) {
				t := float64(x) / math.Max(1, share*float64(canvas.Bounds().Dx()))
				paint = color.NRGBA{uint8(77 * t), uint8(143 - 16*t), uint8(150 + 70*t), 255}
			}
			canvas.SetNRGBA(x, y, paint)
		}
	}
	return emailPNGTag(canvas, width, height, alt, class, true)
}

// Equal-height canvases keep the shared baseline and count scale in email clients.
func emailActivityColumn(count, peak, series int, alt string) template.HTML {
	const width, height = 160, 120
	canvas := image.NewNRGBA(image.Rect(0, 0, width*emailChartScale, height*emailChartScale))
	barHeight := int(104 * float64(max(0, count)) / float64(max(1, peak)))
	paint := emailImagePalette[series%len(emailImagePalette)]
	for y := 0; y < height*emailChartScale; y++ {
		for x := 0; x < width*emailChartScale; x++ {
			if y >= 116*emailChartScale && y < 117*emailChartScale {
				canvas.SetNRGBA(x, y, emailImageTrack)
			}
			if x >= 48*emailChartScale && x < 112*emailChartScale && y >= (116-barHeight)*emailChartScale && y < 116*emailChartScale {
				canvas.SetNRGBA(x, y, paint)
			}
		}
	}
	return emailPNGTag(canvas, width, height, alt, "email-activity-bar", false)
}
