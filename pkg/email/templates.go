package email

// ─── Data structs ─────────────────────────────────────────────────────────────
type EmailDetail struct {
	Label string
	Value string
}
type SubmitConfirmData struct {
	CustomerName string
	ProductName  string
	AppID        string
	Details      []EmailDetail
	LogoURI      string
}

type ApprovalData struct {
	CustomerName string
	ProductName  string
	Details      []EmailDetail
	LogoURI      string
}

type RejectionData struct {
	CustomerName string
	ProductName  string
	Reason       string
	Details      []EmailDetail
	LogoURI      string
}

type ESignLinkData struct {
	CustomerName string
	ProductName  string
	SignLink     string
	Deadline     string
	Details      []EmailDetail
	LogoURI      string
	AppID        string
}

// ─── Shared styles ────────────────────────────────────────────────────────────

const emailCSS = `
  *{box-sizing:border-box;margin:0;padding:0}
  body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;
       background:#f0f4f8;margin:0;padding:0}
  .outer{width:100%;background:#f0f4f8;padding:40px 16px}
  .wrap{max-width:600px;margin:0 auto;background:#fff;border-radius:12px;
        overflow:hidden;box-shadow:0 4px 24px rgba(0,0,0,.10)}
  .hdr{background:linear-gradient(135deg,#1a3f8f 0%,#1a56b0 60%,#2271c3 100%);
       padding:32px 40px;text-align:center;border-bottom:4px solid #f59e0b}
  .hdr img{height:54px;width:auto;display:block;margin:0 auto 12px}
  .hdr-name{font-size:22px;font-weight:800;color:#fff;margin-bottom:4px}
  .hdr-sub{font-size:13px;color:rgba(255,255,255,.80)}
  .bdy{padding:40px 40px 32px}
  .greet{font-size:15px;color:#374151;margin-bottom:8px;line-height:1.6}
  .greet strong{color:#111827}
  .intro{font-size:15px;color:#4b5563;line-height:1.7;margin-bottom:24px}
  .banner{border-radius:10px;padding:18px 22px;margin:24px 0;
          display:flex;align-items:center;gap:14px}
  .b-icon{width:44px;height:44px;border-radius:50%;display:flex;
          align-items:center;justify-content:center;font-size:20px;flex-shrink:0}
  .b-title{font-size:15px;font-weight:700;margin-bottom:2px}
  .b-desc{font-size:13px;opacity:.85}
  .pending{background:#fffbeb;border:1px solid #fde68a;color:#92400e}
  .pending .b-icon{background:#fef3c7}
  .success{background:#f0fdf4;border:1px solid #86efac;color:#14532d}
  .success .b-icon{background:#dcfce7}
  .rejected{background:#fff1f2;border:1px solid #fecdd3;color:#881337}
  .rejected .b-icon{background:#ffe4e6}
  .signing{background:#eff6ff;border:1px solid #93c5fd;color:#1e3a5f}
  .signing .b-icon{background:#dbeafe}
  table.info{width:100%;border-collapse:collapse;border-radius:10px;
             overflow:hidden;border:1px solid #e5e7eb;margin:20px 0;font-size:14px}
  table.info tr:not(:last-child) td{border-bottom:1px solid #f3f4f6}
  table.info td{padding:12px 16px;vertical-align:middle}
  table.info td:first-child{color:#6b7280;font-weight:500;
                            background:#f9fafb;width:42%}
  table.info td:last-child{color:#111827;font-weight:600}
  .cta-wrap{text-align:center;margin:32px 0}
  .cta{display:inline-block;
       background:linear-gradient(135deg,#1a3f8f,#1a56b0);
       color:#fff!important;text-decoration:none;
       padding:14px 44px;border-radius:8px;font-size:16px;font-weight:700;
       box-shadow:0 4px 14px rgba(26,86,176,.35);letter-spacing:.3px}
  .fallback{margin-top:14px;font-size:12px;color:#9ca3af;
            line-height:1.6;word-break:break-all;text-align:center}
  .divider{height:1px;background:#f3f4f6;margin:28px 0}
  .note{background:#f8fafc;border-left:3px solid #1a56b0;
        border-radius:0 8px 8px 0;padding:14px 18px;
        margin:20px 0;font-size:13px;color:#4b5563;line-height:1.6}
  .closing{font-size:14px;color:#6b7280;margin-top:24px;line-height:1.6}
  .closing strong{color:#374151}
  .ftr{background:#1e293b;padding:28px 40px;text-align:center}
  .ftr-name{font-size:14px;font-weight:700;color:#f1f5f9;margin-bottom:4px}
  .ftr-tag{font-size:11px;color:#64748b;margin-bottom:14px;
           letter-spacing:.5px;text-transform:uppercase}
  .ftr a{color:#94a3b8;text-decoration:none;font-size:12px;margin:0 8px}
  .ftr-line{height:1px;background:#334155;margin:14px 0}
  .ftr-legal{font-size:11px;color:#475569;line-height:1.6}
  @media(max-width:600px){.outer{padding:16px 8px}.bdy{padding:28px 24px}
    .hdr{padding:24px}.ftr{padding:24px}}`

const emailOpen = `<!DOCTYPE html><html lang="id"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>BPR Perdana</title><style>` + emailCSS + `</style></head>
<body><div class="outer"><div class="wrap">`

const emailClose = `</div></div></body></html>`

const emailFooter = `<div class="ftr">
  <div class="ftr-name">PT BPR Daya Perdana Nusantara</div>
  <div class="ftr-tag">Melayani dengan Sepenuh Hati</div>
  <a href="mailto:cs@bprperdana.co.id">cs@bprperdana.co.id</a>
  <a href="https://www.bprperdana.co.id">www.bprperdana.co.id</a>
  <div class="ftr-line"></div>
  <div class="ftr-legal">
    Email ini dikirim otomatis. Mohon tidak membalas email ini.<br>
    &copy; 2026 PT BPR Daya Perdana Nusantara. Semua hak dilindungi.
  </div>
</div>`

// ─── Template: Submit Confirmation ───────────────────────────────────────────

const TmplSubmitConfirm = emailOpen + `
<div class="hdr">
  {{if .LogoURI}}<img src="{{.LogoURI}}" alt="BPR Perdana">{{else}}<div class="hdr-name">BPR Perdana</div>{{end}}
  <div class="hdr-sub">Konfirmasi Pengajuan</div>
</div>
<div class="bdy">
  <p class="greet">Halo, <strong>{{.CustomerName}}</strong> &#128075;</p>
  <p class="intro">Terima kasih telah mempercayakan kebutuhan perbankan Anda kepada
    BPR Perdana. Pengajuan Anda telah berhasil kami terima.</p>

  <div class="banner pending">
    <div class="b-icon">&#9203;</div>
    <div>
      <div class="b-title">Pengajuan Diterima</div>
      <div class="b-desc">Sedang menunggu review oleh tim kami</div>
    </div>
  </div>

  <table class="info">
    <tr><td>No. Referensi</td>
        <td style="font-family:monospace;letter-spacing:.5px">{{.AppID}}</td></tr>
    <tr><td>Produk</td><td>{{.ProductName}}</td></tr>
    {{range .Details}}
    <tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>
    {{end}}
    <tr><td>Estimasi Review</td><td>1 – 3 hari kerja</td></tr>
  </table>

  <div class="note">
    &#128161; <strong>Simpan nomor referensi Anda.</strong> Gunakan nomor ini
    untuk mengecek status pengajuan kapan saja melalui halaman
    <em>Cek Status</em> di website kami.
  </div>

  <div class="divider"></div>
  <p class="closing">
    Kami akan mengirimkan notifikasi segera setelah proses review selesai.
    Jika ada pertanyaan, silakan hubungi tim kami.<br><br>
    Salam hangat,<br><strong>Tim BPR Perdana</strong>
  </p>
</div>
` + emailFooter + emailClose

// ─── Template: Approval ───────────────────────────────────────────────────────

const TmplApproval = emailOpen + `
<div class="hdr">
  {{if .LogoURI}}<img src="{{.LogoURI}}" alt="BPR Perdana">{{else}}<div class="hdr-name">BPR Perdana</div>{{end}}
  <div class="hdr-sub">Pengajuan Disetujui &#127881;</div>
</div>
<div class="bdy">
  <p class="greet">Selamat, <strong>{{.CustomerName}}</strong>!</p>
  <p class="intro">Kami dengan bangga menginformasikan bahwa pengajuan Anda
    telah melalui proses review dan mendapatkan <strong>persetujuan</strong>.</p>

  <div class="banner success">
    <div class="b-icon">&#10003;</div>
    <div>
      <div class="b-title">Pengajuan Disetujui</div>
      <div class="b-desc">Kontrak Anda sedang dipersiapkan</div>
    </div>
  </div>

  <table class="info">
    <tr><td>No. Referensi</td>
        <td style="font-family:monospace;letter-spacing:.5px">{{.AppID}}</td></tr>
    <tr><td>Produk</td><td>{{.ProductName}}</td></tr>
    {{range .Details}}
    <tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>
    {{end}}
    <tr><td>Status</td>
        <td style="color:#14532d;font-weight:700">&#10003; Disetujui</td></tr>
    <tr><td>Langkah Berikutnya</td><td>Penandatanganan Kontrak (eSign)</td></tr>
  </table>

  <div class="note">
    &#128233; Tim kami sedang mempersiapkan dokumen kontrak digital Anda.
    Anda akan segera menerima email terpisah berisi <strong>link penandatanganan
    kontrak secara elektronik (eSign)</strong>. Harap cek inbox Anda.
  </div>

  <div class="divider"></div>
  <p class="closing">
    Terima kasih atas kepercayaan Anda. Kami berkomitmen memberikan
    layanan perbankan terbaik untuk Anda.<br><br>
    Salam hangat,<br><strong>Tim BPR Perdana</strong>
  </p>
</div>
` + emailFooter + emailClose

// ─── Template: Rejection ──────────────────────────────────────────────────────

const TmplRejection = emailOpen + `
<div class="hdr">
  {{if .LogoURI}}<img src="{{.LogoURI}}" alt="BPR Perdana">{{else}}<div class="hdr-name">BPR Perdana</div>{{end}}
  <div class="hdr-sub">Pemberitahuan Pengajuan</div>
</div>
<div class="bdy">
  <p class="greet">Halo, <strong>{{.CustomerName}}</strong></p>
  <p class="intro">Terima kasih atas kepercayaan Anda kepada BPR Perdana.
    Setelah melalui proses review, kami perlu menyampaikan informasi
    berikut mengenai pengajuan Anda.</p>

  <div class="banner rejected">
    <div class="b-icon">&#215;</div>
    <div>
      <div class="b-title">Pengajuan Tidak Dapat Diproses</div>
      <div class="b-desc">Produk: {{.ProductName}}</div>
    </div>
  </div>

  <table class="info">
    <tr><td>No. Referensi</td>
        <td style="font-family:monospace;letter-spacing:.5px">{{.AppID}}</td></tr>
    <tr><td>Produk</td><td>{{.ProductName}}</td></tr>
    {{range .Details}}
    <tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>
    {{end}}
    <tr><td>Status</td>
        <td style="color:#881337;font-weight:700">Tidak Disetujui</td></tr>
    <tr><td>Alasan</td>
        <td style="color:#374151">{{.Reason}}</td></tr>
  </table>

  <div class="note">
    &#128172; Mohon maaf atas ketidaknyamanan ini. Anda dapat mengajukan
    kembali setelah melengkapi persyaratan yang diperlukan, atau hubungi
    tim CS kami untuk mendapatkan panduan lebih lanjut.
  </div>

  <div class="cta-wrap">
    <a href="mailto:cs@bprperdana.co.id" class="cta">Hubungi Customer Service</a>
  </div>

  <div class="divider"></div>
  <p class="closing">
    Kami tetap terbuka untuk membantu Anda di masa mendatang.<br><br>
    Salam hormat,<br><strong>Tim BPR Perdana</strong>
  </p>
</div>
` + emailFooter + emailClose

// ─── Template: eSign Link ─────────────────────────────────────────────────────

const TmplESignLink = emailOpen + `
<div class="hdr">
  {{if .LogoURI}}<img src="{{.LogoURI}}" alt="BPR Perdana">{{else}}<div class="hdr-name">BPR Perdana</div>{{end}}
  <div class="hdr-sub">Penandatanganan Kontrak Digital</div>
</div>
<div class="bdy">
  <p class="greet">Halo, <strong>{{.CustomerName}}</strong>!</p>
  <p class="intro">Pengajuan <strong>{{.ProductName}}</strong> Anda telah
    disetujui. Dokumen kontrak digital Anda sudah siap dan menunggu
    tanda tangan elektronik Anda.</p>

  <div class="banner signing">
    <div class="b-icon">&#9999;</div>
    <div>
      <div class="b-title">Kontrak Siap Ditandatangani</div>
      <div class="b-desc">Selesaikan proses dalam batas waktu yang ditentukan</div>
    </div>
  </div>

  <table class="info">
    <tr><td>No. Referensi</td>
        <td style="font-family:monospace;letter-spacing:.5px">{{.AppID}}</td></tr>
    <tr><td>Produk</td><td>{{.ProductName}}</td></tr>
    {{range .Details}}
    <tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>
    {{end}}
    <tr><td>Batas Waktu TTD</td>
        <td style="color:#dc2626;font-weight:700">{{.Deadline}}</td></tr>
  </table>

  <div class="cta-wrap">
    <a href="{{.SignLink}}" class="cta">&#9998;&nbsp; Tanda Tangani Sekarang</a>
    <div class="fallback">
      Jika tombol di atas tidak berfungsi, copy dan paste URL ini ke browser:<br>
      <span style="color:#1a56b0">{{.SignLink}}</span>
    </div>
  </div>

  <div class="note">
    &#9888;&#65039; <strong>Perhatian:</strong> Link penandatanganan ini bersifat
    rahasia dan hanya untuk Anda. Jangan bagikan kepada siapapun.
    Link akan kedaluwarsa pada <strong>{{.Deadline}}</strong>.
  </div>

  <div class="divider"></div>
  <p class="closing">
    Jika Anda mengalami kendala dalam proses penandatanganan, segera
    hubungi tim kami sebelum batas waktu berakhir.<br><br>
    Salam hangat,<br><strong>Tim BPR Perdana</strong>
  </p>
</div>
` + emailFooter + emailClose
