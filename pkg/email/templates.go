package email

// ── Template data structs ─────────────────────────────────────────────────────

type SubmitConfirmData struct {
	CustomerName string
	ProductName  string
	AppID        string // short ID untuk ditampilkan
}

type RejectionData struct {
	CustomerName string
	ProductName  string
	Reason       string
}

type ApprovalData struct {
	CustomerName string
	ProductName  string
}

type ESignLinkData struct {
	CustomerName string
	ProductName  string
	SignLink     string
	Deadline     string // format: "7 April 2026"
}

// ── Template strings ──────────────────────────────────────────────────────────

const tmplBase = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
  body { font-family: Arial, sans-serif; background: #f4f6f9; margin: 0; padding: 0; }
  .wrapper { max-width: 600px; margin: 40px auto; background: #fff;
             border-radius: 8px; overflow: hidden;
             box-shadow: 0 2px 8px rgba(0,0,0,0.08); }
  .header { background: #1a56b0; color: #fff; padding: 28px 32px; }
  .header h1 { margin: 0; font-size: 20px; }
  .header p { margin: 4px 0 0; font-size: 13px; opacity: 0.85; }
  .body { padding: 28px 32px; color: #374151; line-height: 1.6; }
  .body h2 { font-size: 16px; color: #111827; margin-top: 0; }
  .info-box { background: #f9fafb; border: 1px solid #e5e7eb;
              border-radius: 6px; padding: 16px; margin: 16px 0; }
  .info-row { display: flex; justify-content: space-between;
              padding: 6px 0; border-bottom: 1px solid #e5e7eb; font-size: 14px; }
  .info-row:last-child { border-bottom: none; }
  .info-label { color: #6b7280; }
  .info-value { font-weight: 600; color: #111827; }
  .badge { display: inline-block; padding: 4px 12px; border-radius: 20px;
           font-size: 13px; font-weight: 600; }
  .badge-success { background: #d1fae5; color: #065f46; }
  .badge-rejected { background: #fee2e2; color: #991b1b; }
  .badge-pending { background: #fef3c7; color: #92400e; }
  .footer { background: #f9fafb; padding: 20px 32px;
            font-size: 12px; color: #9ca3af; text-align: center;
            border-top: 1px solid #e5e7eb; }
</style>
</head>
<body><div class="wrapper">{{.Content}}</div></body>
</html>`

// TmplSubmitConfirm — dikirim saat nasabah submit pengajuan
const TmplSubmitConfirm = `
<div class="header">
  <h1>BPR Perdana</h1>
  <p>Konfirmasi Pengajuan</p>
</div>
<div class="body">
  <h2>Pengajuan Anda Telah Diterima</h2>
  <p>Halo <strong>{{.CustomerName}}</strong>,</p>
  <p>Terima kasih telah mengajukan permohonan pembukaan rekening di BPR Perdana.
     Pengajuan Anda sedang dalam proses review oleh tim kami.</p>
  <div class="info-box">
    <div class="info-row">
      <span class="info-label">Produk</span>
      <span class="info-value">{{.ProductName}}</span>
    </div>
    <div class="info-row">
      <span class="info-label">No. Referensi</span>
      <span class="info-value">{{.AppID}}</span>
    </div>
    <div class="info-row">
      <span class="info-label">Status</span>
      <span class="badge badge-pending">Menunggu Review</span>
    </div>
  </div>
  <p>Kami akan menginformasikan hasil review melalui email ini.
     Proses review biasanya memakan waktu 1–3 hari kerja.</p>
  <p>Jika ada pertanyaan, hubungi kami di
     <a href="mailto:cs@bprperdana.co.id">cs@bprperdana.co.id</a>.</p>
</div>
<div class="footer">
  PT BPR Daya Perdana Nusantara &bull; Email ini dikirim otomatis, jangan balas langsung.
</div>
`

// TmplRejection — dikirim saat pengajuan ditolak
const TmplRejection = `
<div class="header">
  <h1>BPR Perdana</h1>
  <p>Pemberitahuan Pengajuan</p>
</div>
<div class="body">
  <h2>Pengajuan Tidak Dapat Diproses</h2>
  <p>Halo <strong>{{.CustomerName}}</strong>,</p>
  <p>Setelah melakukan review, kami mohon maaf untuk menyampaikan bahwa
     pengajuan Anda untuk produk <strong>{{.ProductName}}</strong>
     tidak dapat kami proses saat ini.</p>
  <div class="info-box">
    <div class="info-row">
      <span class="info-label">Status</span>
      <span class="badge badge-rejected">Tidak Disetujui</span>
    </div>
    <div class="info-row">
      <span class="info-label">Alasan</span>
      <span class="info-value">{{.Reason}}</span>
    </div>
  </div>
  <p>Anda dapat mengajukan kembali setelah melengkapi persyaratan yang diperlukan.
     Untuk informasi lebih lanjut, hubungi kami di
     <a href="mailto:cs@bprperdana.co.id">cs@bprperdana.co.id</a>
     atau kunjungi kantor kami.</p>
</div>
<div class="footer">
  PT BPR Daya Perdana Nusantara &bull; Email ini dikirim otomatis, jangan balas langsung.
</div>
`

// TmplApproval — dikirim saat pengajuan disetujui (sebelum eSign ready)
const TmplApproval = `
<div class="header">
  <h1>BPR Perdana</h1>
  <p>Pengajuan Disetujui</p>
</div>
<div class="body">
  <h2>Selamat! Pengajuan Anda Disetujui</h2>
  <p>Halo <strong>{{.CustomerName}}</strong>,</p>
  <p>Kami dengan senang hati menginformasikan bahwa pengajuan Anda untuk produk
     <strong>{{.ProductName}}</strong> telah <strong>disetujui</strong>.</p>
  <div class="info-box">
    <div class="info-row">
      <span class="info-label">Status</span>
      <span class="badge badge-success">Disetujui</span>
    </div>
  </div>
  <p>Tim kami sedang mempersiapkan dokumen kontrak Anda.
     Anda akan menerima email terpisah berisi link untuk penandatanganan kontrak
     secara elektronik (eSign).</p>
  <p>Jika ada pertanyaan, hubungi kami di
     <a href="mailto:cs@bprperdana.co.id">cs@bprperdana.co.id</a>.</p>
</div>
<div class="footer">
  PT BPR Daya Perdana Nusantara &bull; Email ini dikirim otomatis, jangan balas langsung.
</div>
`
const TmplESignLink = `
<div class="header">
  <h1>BPR Perdana</h1>
  <p>Penandatanganan Kontrak</p>
</div>
<div class="body">
  <h2>Kontrak Siap Ditandatangani</h2>
  <p>Halo <strong>{{.CustomerName}}</strong>,</p>
  <p>Kontrak pembukaan rekening <strong>{{.ProductName}}</strong> Anda telah siap.
     Silakan tandatangani secara elektronik melalui link berikut:</p>
  <div class="info-box" style="text-align:center; padding: 24px;">
    <a href="{{.SignLink}}"
       style="background:#1a56b0;color:#fff;padding:12px 32px;
              border-radius:6px;text-decoration:none;font-weight:600;font-size:15px;">
      Tanda Tangani Sekarang
    </a>
  </div>
  <div class="info-box">
    <div class="info-row">
      <span class="info-label">Batas Waktu</span>
      <span class="info-value" style="color:#dc2626;">{{.Deadline}}</span>
    </div>
  </div>
  <p style="color:#6b7280;font-size:13px;">
    Jika link di atas tidak bisa diklik, copy URL ini ke browser Anda:<br>
    <span style="word-break:break-all;">{{.SignLink}}</span>
  </p>
  <p>Jika ada pertanyaan, hubungi kami di
     <a href="mailto:cs@bprperdana.co.id">cs@bprperdana.co.id</a>.</p>
</div>
<div class="footer">
  PT BPR Daya Perdana Nusantara &bull; Email ini dikirim otomatis, jangan balas langsung.
</div>
`
