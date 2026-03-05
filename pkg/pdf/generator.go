// Package pdf generates contract documents for PT BPR Daya Perdana Nusantara.
//
// Menghasilkan PDF kontrak yang berisi:
//   - Header dengan logo perusahaan + nama + motto
//   - Data nasabah (dari OCR KTP + Step 4)
//   - Detail produk (SAVING / DEPOSIT / LOAN)
//   - Rekening pencairan (khusus DEPOSIT dan LOAN)
//   - Klausul perjanjian
//   - Kolom tanda tangan (3 kolom: nasabah, tanggal, pejabat)
//
// DESAIN:
//
//	Warna merah #D43746 (primary), kuning #FED73B (aksen), abu #3D3D3D (teks)
//	Sesuai brand logo BPR Perdana.
package pdf

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// ─── Brand constants ──────────────────────────────────────────────────────────

const (
	companyFull  = "PT BPR DAYA PERDANA NUSANTARA"
	companyMotto = "Mitra yang Berharga dan Dapat Diandalkan"

	pageW    = 210.0 // A4 width mm
	pageH    = 297.0 // A4 height mm
	marginL  = 18.0
	marginR  = 18.0
	marginT  = 16.0
	marginB  = 14.0
	contentW = pageW - marginL - marginR // 174 mm
)

// Brand RGB colors
var (
	colRed    = [3]int{212, 55, 70}
	colYellow = [3]int{254, 215, 59}
	colDGray  = [3]int{61, 61, 61}
	colLGray  = [3]int{247, 247, 247}
	colMGray  = [3]int{221, 221, 221}
	colWhite  = [3]int{255, 255, 255}
	colBlack  = [3]int{26, 26, 26}
)

// Indonesian months
var monthsID = [13]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

// ─── Data types ───────────────────────────────────────────────────────────────

// ContractData berisi semua data yang diperlukan untuk generate PDF kontrak.
type ContractData struct {
	ApplicationID string
	ProductType   string // "SAVING" | "DEPOSIT" | "LOAN"
	ApprovedAt    time.Time
	LogoPath      string // path ke file logo PNG (opsional)

	// Data nasabah dari OCR KTP
	FullName      string
	NIK           string
	BirthPlace    string
	BirthDate     string
	Address       string
	Kelurahan     string
	Kecamatan     string
	KabupatenKota string
	Provinsi      string
	Occupation    string
	Nationality   string

	// Data dari Step 4 (personal info)
	Email             string
	PhoneNumber       string
	Education         string
	MonthlyIncome     uint64
	MothersMaidenName string

	// Detail produk — hanya salah satu yang diisi
	Saving  *SavingData
	Deposit *DepositData
	Loan    *LoanData

	// Rekening pencairan (DEPOSIT & LOAN)
	BankName      string
	BankCode      string
	AccountNumber string
	AccountHolder string
}

type SavingData struct {
	ProductName    string
	InitialDeposit uint64
	SourceOfFunds  string
	SavingPurpose  string
}

type DepositData struct {
	ProductName       string
	PlacementAmount   uint64
	TenorMonths       uint8
	InterestRate      string // e.g. "6,5% per tahun"
	RolloverType      string
	SourceOfFunds     string
	InvestmentPurpose string
}

type LoanData struct {
	ProductName     string
	RequestedAmount uint64
	TenorMonths     uint8
	LoanPurpose     string
	PaymentSource   string
	SourceOfFunds   string
}

// ─── Generator ────────────────────────────────────────────────────────────────

// GenerateContract membuat PDF kontrak dan mengembalikan bytes-nya.
func GenerateContract(data ContractData) ([]byte, error) {
	g := &generator{
		pdf:  gofpdf.New("P", "mm", "A4", ""),
		data: data,
	}
	return g.build()
}

type generator struct {
	pdf  *gofpdf.Fpdf
	data ContractData
}

func (g *generator) build() ([]byte, error) {
	pdf := g.pdf
	pdf.SetMargins(marginL, marginT, marginR)
	pdf.SetAutoPageBreak(true, marginB)
	pdf.AddPage()

	// Page decorations (stripe top + bottom + border)
	g.drawPageBorder()

	// ── Header ────────────────────────────────────────────────────────────────
	g.drawHeader()

	// ── Judul dokumen ─────────────────────────────────────────────────────────
	pdf.Ln(3)
	g.setFont("B", 11)
	g.setTextColor(colRed)
	pdf.CellFormat(contentW, 7, g.docTitle(), "", 1, "C", false, 0, "")

	docNo := g.docNumber()
	g.setFont("", 8.5)
	g.setTextColor(colDGray)
	pdf.CellFormat(contentW, 5.5, "Nomor: "+docNo, "", 1, "C", false, 0, "")
	pdf.CellFormat(contentW, 5.5, "Tanggal: "+g.dateID(g.data.ApprovedAt), "", 1, "C", false, 0, "")

	// Thin divider
	g.setDrawColor(colMGray)
	pdf.SetLineWidth(0.3)
	pdf.Line(marginL, pdf.GetY()+1.5, pageW-marginR, pdf.GetY()+1.5)
	pdf.Ln(4)

	// ── Sections ──────────────────────────────────────────────────────────────
	g.sectionHeader("DATA NASABAH")
	g.dataRows(g.nasabahRows())

	pdf.Ln(3)
	g.sectionHeader(g.productSectionTitle())
	g.dataRows(g.productRows())

	if g.data.ProductType == "DEPOSIT" || g.data.ProductType == "LOAN" {
		pdf.Ln(3)
		g.sectionHeader("REKENING PENCAIRAN")
		g.dataRows(g.rekeningRows())
	}

	pdf.Ln(3)
	// Cek ruang untuk minimal 3 klausa + header (~40mm)
	if pageH-marginB-marginT-pdf.GetY() < 40 {
		pdf.AddPage()
		g.drawPageBorder()
	}
	g.sectionHeader("PERNYATAAN DAN PERSETUJUAN")
	pdf.Ln(2)
	g.drawClauses()

	pdf.Ln(5)
	// Cek apakah cukup ruang untuk signature section (~55mm)
	// Jika tidak cukup, pindah ke halaman baru
	remainingSpace := pageH - marginB - marginT - pdf.GetY()
	if remainingSpace < 55 {
		pdf.AddPage()
		g.drawPageBorder()
	}
	g.sectionHeader("PENANDATANGANAN")
	pdf.Ln(3)
	g.drawSignature()

	pdf.Ln(4)
	// Footer divider
	g.setDrawColor(colMGray)
	pdf.SetLineWidth(0.3)
	pdf.Line(marginL, pdf.GetY(), pageW-marginR, pdf.GetY())
	pdf.Ln(1)

	g.setFont("I", 6.5)
	g.setTextColor(colMGray)
	footerText := fmt.Sprintf(
		"%s  |  ID: %s  |  %s WIB  |  Ditandatangani secara elektronik melalui VIDA",
		companyFull,
		g.data.ApplicationID,
		time.Now().Format("02-01-2006 15:04"),
	)
	pdf.MultiCell(contentW, 4, footerText, "", "C", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output failed: %w", err)
	}
	return buf.Bytes(), nil
}

// ─── Page decoration ──────────────────────────────────────────────────────────

func (g *generator) drawPageBorder() {
	pdf := g.pdf

	// Red top stripe
	g.setFillColor(colRed)
	pdf.Rect(0, 0, pageW, 9, "F")

	// Yellow bottom stripe
	g.setFillColor(colYellow)
	pdf.Rect(0, pageH-4, pageW, 4, "F")

	// Light border box
	g.setDrawColor(colMGray)
	pdf.SetLineWidth(0.4)
	pdf.Rect(10, 8, pageW-20, pageH-16, "D")

	// Page number (in yellow stripe area)
	g.setFont("", 7)
	g.setTextColor(colDGray)
	pdf.SetXY(0, pageH-3.5)
	pdf.CellFormat(pageW, 3, fmt.Sprintf("Halaman %d", pdf.PageCount()), "", 0, "C", false, 0, "")

	// Reset cursor to content area
	pdf.SetXY(marginL, marginT)
}

// ─── Header ───────────────────────────────────────────────────────────────────

func (g *generator) drawHeader() {
	pdf := g.pdf
	startY := pdf.GetY()

	// ── Logo (kiri) ───────────────────────────────────────────────────────────
	logoW := 58.0
	logoH := 20.0
	logoDrawn := false
	if g.data.LogoPath != "" {
		info := pdf.RegisterImage(g.data.LogoPath, "")
		if info != nil {
			pdf.ImageOptions(g.data.LogoPath, marginL, startY, logoW, logoH,
				false, gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, 0, "")
			logoDrawn = true
		}
	}
	if !logoDrawn {
		// Fallback: teks "BPR Perdana" jika logo tidak ada
		g.setFont("B", 14)
		g.setTextColor(colRed)
		pdf.SetXY(marginL, startY+4)
		pdf.CellFormat(logoW, 8, "BPR perdana", "", 0, "L", false, 0, "")
	}

	// ── Nama perusahaan + motto (kanan) ───────────────────────────────────────
	rightX := marginL + logoW + 3
	rightW := contentW - logoW - 3

	// Nama perusahaan
	g.setFont("B", 9)
	g.setTextColor(colDGray)
	pdf.SetXY(rightX, startY+3)
	pdf.CellFormat(rightW, 6, companyFull, "", 1, "R", false, 0, "")

	// Motto — rata kanan, sejajar dengan nama perusahaan
	g.setFont("I", 8)
	g.setTextColor(colRed)
	pdf.SetXY(rightX, startY+9.5)
	pdf.CellFormat(rightW, 5, companyMotto, "", 1, "R", false, 0, "")

	// Yellow underline
	lineY := startY + logoH + 1
	g.setDrawColor(colYellow)
	pdf.SetLineWidth(1.5)
	pdf.Line(marginL, lineY, pageW-marginR, lineY)
	pdf.SetLineWidth(0.3)

	pdf.SetXY(marginL, lineY+2)
}

// ─── Section header ───────────────────────────────────────────────────────────

func (g *generator) sectionHeader(title string) {
	pdf := g.pdf
	g.setFillColor(colRed)
	g.setTextColor(colWhite)
	g.setFont("B", 9)
	pdf.CellFormat(contentW, 7, "  "+title, "", 1, "L", true, 0, "")
}

// ─── Data table ───────────────────────────────────────────────────────────────

const (
	colLabel = 55.0
	colColon = 5.0
	colValue = contentW - colLabel - colColon
)

func (g *generator) dataRows(rows [][2]string) {
	pdf := g.pdf
	for i, row := range rows {
		// Alternating row background
		if i%2 == 0 {
			g.setFillColor(colWhite)
		} else {
			g.setFillColor(colLGray)
		}

		// Calculate height based on value length (for multiline)
		lineH := 5.5
		lines := pdf.SplitLines([]byte(row[1]), colValue-2)
		rowH := float64(len(lines)) * lineH
		if rowH < lineH {
			rowH = lineH
		}

		rowY := pdf.GetY()

		// Label
		g.setFont("B", 8.5)
		g.setTextColor(colDGray)
		pdf.SetX(marginL)
		pdf.CellFormat(colLabel, rowH, row[0], "", 0, "LM", true, 0, "")

		// Colon
		pdf.CellFormat(colColon, rowH, ":", "", 0, "CM", true, 0, "")

		// Value (MultiCell for wrapping)
		g.setFont("", 8.5)
		g.setTextColor(colBlack)
		pdf.SetXY(marginL+colLabel+colColon, rowY)
		pdf.MultiCell(colValue, lineH, row[1], "", "LM", true)

		// Ensure cursor is after the row
		if pdf.GetY() < rowY+rowH {
			pdf.SetY(rowY + rowH)
		}
	}
	// Bottom border on last row
	g.setDrawColor(colMGray)
	pdf.Line(marginL, pdf.GetY(), pageW-marginR, pdf.GetY())
}

// ─── Clauses ──────────────────────────────────────────────────────────────────

func (g *generator) drawClauses() {
	pdf := g.pdf
	clauses := g.buildClauses()
	g.setFont("", 8)
	g.setTextColor(colDGray)
	for i, c := range clauses {
		pdf.SetX(marginL)
		pdf.MultiCell(contentW, 5, fmt.Sprintf("%d.  %s", i+1, c), "", "J", false)
		pdf.Ln(1)
	}
}

// ─── Signature table ──────────────────────────────────────────────────────────

func (g *generator) drawSignature() {
	pdf := g.pdf
	colW := contentW / 3

	headers := []string{"Nasabah", "Tanggal Penandatanganan", "PT BPR Daya Perdana Nusantara"}
	names := []string{g.data.FullName, "____________________", "Pejabat Berwenang"}

	startY := pdf.GetY()

	// Header row (red background)
	g.setFillColor(colRed)
	g.setTextColor(colWhite)
	g.setFont("B", 8.5)
	for i, h := range headers {
		pdf.SetXY(marginL+float64(i)*colW, startY)
		pdf.CellFormat(colW, 8, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(8)

	// Signature space row
	g.setFillColor(colWhite)
	sigY := pdf.GetY()
	for i := 0; i < 3; i++ {
		pdf.SetXY(marginL+float64(i)*colW, sigY)
		pdf.CellFormat(colW, 24, "", "LR", 0, "C", false, 0, "")
	}
	pdf.Ln(24)

	// Name row
	g.setFont("B", 8.5)
	nameY := pdf.GetY()
	for i, n := range names {
		pdf.SetXY(marginL+float64(i)*colW, nameY)
		isBold := i == 0
		if isBold {
			g.setFont("B", 8.5)
			g.setTextColor(colBlack)
		} else {
			g.setFont("", 7.5)
			g.setTextColor(colDGray)
		}
		pdf.CellFormat(colW, 8, n, "LRB", 0, "C", false, 0, "")
	}
	pdf.Ln(8)
}

// ─── Content helpers ──────────────────────────────────────────────────────────

func (g *generator) docTitle() string {
	switch g.data.ProductType {
	case "SAVING":
		return "PERJANJIAN PEMBUKAAN REKENING TABUNGAN"
	case "DEPOSIT":
		return "PERJANJIAN PENEMPATAN DEPOSITO"
	case "LOAN":
		return "PERJANJIAN KREDIT"
	default:
		return "PERJANJIAN LAYANAN PERBANKAN"
	}
}

func (g *generator) docNumber() string {
	var prefix string
	switch g.data.ProductType {
	case "SAVING":
		prefix = "TAB"
	case "DEPOSIT":
		prefix = "DEP"
	case "LOAN":
		prefix = "KRD"
	default:
		prefix = "DOC"
	}
	appShort := g.data.ApplicationID
	if len(appShort) > 8 {
		appShort = appShort[:8]
	}
	return fmt.Sprintf("%s/PKP/%s/%s", prefix, g.data.ApprovedAt.Format("01/2006"), appShort)
}

func (g *generator) dateID(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), monthsID[t.Month()], t.Year())
}

func (g *generator) productSectionTitle() string {
	switch g.data.ProductType {
	case "SAVING":
		return "DETAIL TABUNGAN"
	case "DEPOSIT":
		return "DETAIL DEPOSITO"
	case "LOAN":
		return "DETAIL KREDIT"
	default:
		return "DETAIL PRODUK"
	}
}

func (g *generator) nasabahRows() [][2]string {
	d := g.data
	return [][2]string{
		{"Nama Lengkap", d.FullName},
		{"NIK", d.NIK},
		{"Tempat / Tgl Lahir", d.BirthPlace + ", " + formatDate(d.BirthDate)},
		{"Alamat", d.Address},
		{"Kelurahan / Desa", d.Kelurahan},
		{"Kecamatan", d.Kecamatan},
		{"Kab / Kota", d.KabupatenKota},
		{"Provinsi", d.Provinsi},
		{"Pekerjaan", d.Occupation},
		{"Kewarganegaraan", d.Nationality},
		{"No. HP", d.PhoneNumber},
		{"Email", d.Email},
		{"Pendidikan", d.Education},
		{"Nama Ibu Kandung", d.MothersMaidenName},
		{"Penghasilan / Bulan", "Rp " + formatIDR(d.MonthlyIncome)},
	}
}

func (g *generator) productRows() [][2]string {
	switch g.data.ProductType {
	case "SAVING":
		if g.data.Saving == nil {
			return nil
		}
		s := g.data.Saving
		return [][2]string{
			{"Nama Produk", s.ProductName},
			{"Setoran Awal", "Rp " + formatIDR(s.InitialDeposit)},
			{"Sumber Dana", s.SourceOfFunds},
			{"Tujuan Menabung", s.SavingPurpose},
		}
	case "DEPOSIT":
		if g.data.Deposit == nil {
			return nil
		}
		dep := g.data.Deposit
		rate := dep.InterestRate
		if rate == "" {
			rate = "-"
		}
		return [][2]string{
			{"Nama Produk", dep.ProductName},
			{"Nominal Penempatan", "Rp " + formatIDR(dep.PlacementAmount)},
			{"Tenor", fmt.Sprintf("%d bulan", dep.TenorMonths)},
			{"Suku Bunga", rate},
			{"Perpanjangan", dep.RolloverType},
			{"Sumber Dana", dep.SourceOfFunds},
			{"Tujuan Investasi", dep.InvestmentPurpose},
		}
	case "LOAN":
		if g.data.Loan == nil {
			return nil
		}
		l := g.data.Loan
		return [][2]string{
			{"Nama Produk", l.ProductName},
			{"Jumlah Pinjaman", "Rp " + formatIDR(l.RequestedAmount)},
			{"Tenor", fmt.Sprintf("%d bulan", l.TenorMonths)},
			{"Tujuan Kredit", l.LoanPurpose},
			{"Sumber Pembayaran", l.PaymentSource},
			{"Sumber Dana", l.SourceOfFunds},
		}
	}
	return nil
}

func (g *generator) rekeningRows() [][2]string {
	d := g.data
	return [][2]string{
		{"Nama Bank", d.BankName},
		{"Kode Bank", d.BankCode},
		{"No. Rekening", d.AccountNumber},
		{"Atas Nama", d.AccountHolder},
	}
}

func (g *generator) buildClauses() []string {
	base := []string{
		"Dengan ini saya menyatakan bahwa seluruh data yang saya sampaikan adalah benar, lengkap, dan dapat dipertanggungjawabkan.",
		"Saya memberikan persetujuan kepada PT BPR Daya Perdana Nusantara untuk melakukan verifikasi data kepada Direktorat Jenderal Kependudukan dan Pencatatan Sipil (Dukcapil) sesuai ketentuan yang berlaku.",
		"Saya memahami dan menyetujui Syarat dan Ketentuan Umum PT BPR Daya Perdana Nusantara yang telah saya baca sebelum mengajukan permohonan ini.",
		"Saya menyetujui bahwa dokumen ini ditandatangani secara elektronik dan memiliki kekuatan hukum yang setara dengan tanda tangan basah sesuai UU ITE No. 11 Tahun 2008 beserta perubahannya.",
		"Saya bertanggung jawab atas keamanan data akses rekening saya dan tidak akan membagikannya kepada pihak lain.",
	}
	switch g.data.ProductType {
	case "LOAN":
		return append(base,
			"Saya memahami bahwa keterlambatan pembayaran angsuran akan dikenakan denda sesuai ketentuan yang berlaku.",
			"Saya menyetujui bahwa PT BPR Daya Perdana Nusantara berhak melakukan penagihan melalui berbagai saluran komunikasi yang telah saya daftarkan.",
			"Saya memahami bahwa fasilitas kredit ini tunduk pada peraturan Otoritas Jasa Keuangan (OJK) yang berlaku.",
		)
	case "DEPOSIT":
		return append(base,
			"Saya memahami bahwa pencairan deposito sebelum jatuh tempo akan dikenakan penalti sesuai ketentuan yang berlaku.",
			"Saya memahami bahwa bunga deposito dikenakan pajak penghasilan sesuai peraturan perpajakan yang berlaku.",
			"Saya menyetujui perpanjangan otomatis sesuai ketentuan ARO yang saya pilih.",
			"Saya memahami bahwa simpanan dijamin oleh Lembaga Penjamin Simpanan (LPS) sesuai ketentuan yang berlaku.",
		)
	case "SAVING":
		return append(base,
			"Saya memahami bahwa rekening tabungan ini tunduk pada Syarat dan Ketentuan Tabungan PT BPR Daya Perdana Nusantara yang berlaku.",
			"Saya menyetujui bahwa PT BPR Daya Perdana Nusantara berhak memblokir atau menutup rekening apabila ditemukan indikasi penyalahgunaan sesuai ketentuan hukum.",
			"Saya memahami bahwa simpanan dijamin oleh Lembaga Penjamin Simpanan (LPS) sesuai ketentuan yang berlaku.",
		)
	}
	return base
}

// ─── Color/font helpers ───────────────────────────────────────────────────────

func (g *generator) setFillColor(c [3]int) {
	g.pdf.SetFillColor(c[0], c[1], c[2])
}
func (g *generator) setTextColor(c [3]int) {
	g.pdf.SetTextColor(c[0], c[1], c[2])
}
func (g *generator) setDrawColor(c [3]int) {
	g.pdf.SetDrawColor(c[0], c[1], c[2])
}
func (g *generator) setFont(style string, size float64) {
	g.pdf.SetFont("Arial", style, size)
}

// ─── Date formatting ─────────────────────────────────────────────────────────

// formatDate mengkonversi berbagai format tanggal ke DD-MM-YYYY.
// Handles: "2006-01-02", "2006-01-02T15:04:05Z07:00", "02-01-2006"
func formatDate(s string) string {
	if s == "" {
		return ""
	}
	// Coba parse ISO datetime dulu (MySQL DATE kadang dikembalikan sebagai datetime)
	formats := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02-01-2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return fmt.Sprintf("%02d-%02d-%d", t.Day(), int(t.Month()), t.Year())
		}
	}
	// Jika tidak bisa di-parse, kembalikan apa adanya
	return s
}

// ─── Number formatting ────────────────────────────────────────────────────────

// formatIDR memformat angka ke format ribuan Rupiah.
// Contoh: 100000000 → "100.000.000"
func formatIDR(amount uint64) string {
	if amount == 0 {
		return "0"
	}
	s := fmt.Sprintf("%d", amount)
	var b strings.Builder
	start := len(s) % 3
	if start > 0 {
		b.WriteString(s[:start])
	}
	for i := start; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
