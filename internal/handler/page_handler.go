package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"html/template"
	"net"
	"os"
	"strings"
	"sync"

	"ispsync/internal/domain"
	"ispsync/internal/middleware"
	"ispsync/internal/repository"
)

type PageHandler struct {
	store     repository.Storage
	htmlCache string
	mu        sync.RWMutex
}

func NewPageHandler(store repository.Storage) *PageHandler {
	content, err := os.ReadFile("web/index.html")
	if err != nil {
		content, _ = os.ReadFile("./index.html")
	}
	return &PageHandler{
		store:     store,
		htmlCache: string(content),
	}
}

func (h *PageHandler) ServeApp(w http.ResponseWriter, r *http.Request) {
	tCtx := middleware.GetTenantContext(r)
	if tCtx == nil {
		http.Error(w, "Tenant not found", http.StatusNotFound)
		return
	}

	if tCtx.AppType == domain.AppWifi {
		h.ServeHotspot(w, r)
		return
	}
	if tCtx.AppType == domain.AppIsolir {
		h.ServeIsolir(w, r)
		return
	}

	// Guard against serving HTML as a corrupted PDF if requested directly
	if strings.HasSuffix(strings.ToLower(r.URL.Path), ".pdf") {
		http.NotFound(w, r)
		return
	}

	h.mu.RLock()
	html := h.htmlCache
	h.mu.RUnlock()

	if html == "" {
		b, err := os.ReadFile("web/index.html")
		if err == nil {
			html = string(b)
			h.mu.Lock()
			h.htmlCache = html
			h.mu.Unlock()
		}
	}

	allTenants, _ := h.store.ListTenants(r.Context())
	allTenantsJSON, _ := json.Marshal(allTenants)

	// Determine active UI mode from subdomain / appType
	mode := "unified"
	switch tCtx.AppType {
	case domain.AppSales:
		mode = "sales"
	case domain.AppTeknisi:
		mode = "technician"
	case domain.AppNOC:
		mode = "noc-fo"
	case domain.AppCMS:
		mode = "noc-fo"
	case domain.AppPortal:
		mode = "portal"
	}

	// 2. Injected Early Tenant Context Script
	brandShort := tCtx.Tenant.PrefixID
	if brandShort == "" || strings.HasPrefix(strings.ToUpper(brandShort), "PT") || brandShort == "DEV" {
		brandShort = strings.ToUpper(tCtx.Tenant.Slug)
	}
	if brandShort == "" || brandShort == "DEV" {
		brandShort = "ISPSYNC"
	}

	var pageTitle string
	switch mode {
	case "sales":
		pageTitle = fmt.Sprintf("Masuk | %s Sales", brandShort)
	case "noc-fo":
		pageTitle = fmt.Sprintf("Masuk | %s NOC Command Center", brandShort)
	case "technician":
		pageTitle = fmt.Sprintf("Masuk | %s Portal Teknisi", brandShort)
	case "portal":
		pageTitle = fmt.Sprintf("%s • Portal Pelanggan & Jangkauan Fiber", brandShort)
	default:
		pageTitle = fmt.Sprintf("%s • Registrasi & Jangkauan Fiber Optik", brandShort)
	}

	// 1. Topbar Tenant Switcher
	var switcherBtns strings.Builder
	for _, t := range allTenants {
		activeStyle := "color:#94a3b8; background:transparent;"
		if t.Slug == tCtx.Tenant.Slug {
			activeStyle = fmt.Sprintf("color:%s; background:#1e293b; font-weight:bold; border:1px solid #334155;", t.BrandColor)
		}
		btnURL := fmt.Sprintf("https://%s.%s.ispsync.id", tCtx.AppType, t.Slug)
		switcherBtns.WriteString(fmt.Sprintf(
			`<a href="%s" style="%s text-decoration:none; padding:2px 8px; border-radius:4px; margin-left:4px; font-size:11px;">[%s %s]</a>`,
			btnURL, activeStyle, t.PrefixID, t.Name,
		))
	}

	topbarHTML := fmt.Sprintf(`
<div id="ispsync-topbar" style="background:#090d16; color:#e2e8f0; font-family:'JetBrains Mono', monospace; font-size:11px; padding:6px 16px; border-bottom:1px solid #1e293b; display:flex; justify-content:space-between; align-items:center; position:sticky; top:0; z-index:999999;">
  <div style="display:flex; align-items:center; gap:8px;">
    <span style="background:#10b981; color:#064e3b; font-weight:800; padding:1px 6px; border-radius:4px; font-size:10px;">ISPSYNC SAAS</span>
    <span>Tenant: <strong style="color:%s;">%s</strong> (%s)</span>
    <span style="color:#475569;">|</span>
    <span style="color:#94a3b8;">App: <strong style="color:#38bdf8;">%s</strong></span>
  </div>
  <div style="display:flex; align-items:center; gap:4px;">
    <span style="color:#64748b; font-size:11px;">Tenant Switcher:</span>
    %s
  </div>
</div>
`, tCtx.Tenant.BrandColor, tCtx.Tenant.Name, tCtx.Tenant.PrefixID, strings.ToUpper(string(tCtx.AppType)), switcherBtns.String())

	phoneRaw := tCtx.Tenant.ContactPhone
	if phoneRaw == "" {
		phoneRaw = "081100002026"
	}
	phoneWA := phoneRaw
	if strings.HasPrefix(phoneWA, "0") {
		phoneWA = "62" + phoneWA[1:]
	} else if strings.HasPrefix(phoneWA, "+62") {
		phoneWA = phoneWA[1:]
	}
	phoneDisplay := phoneRaw
	if len(phoneRaw) == 12 && strings.HasPrefix(phoneRaw, "08") {
		phoneDisplay = fmt.Sprintf("%s-%s-%s", phoneRaw[:4], phoneRaw[4:8], phoneRaw[8:])
	} else if len(phoneRaw) == 11 && strings.HasPrefix(phoneRaw, "08") {
		phoneDisplay = fmt.Sprintf("%s-%s-%s", phoneRaw[:4], phoneRaw[4:7], phoneRaw[7:])
	} else if len(phoneRaw) == 13 && strings.HasPrefix(phoneRaw, "08") {
		phoneDisplay = fmt.Sprintf("%s-%s-%s", phoneRaw[:4], phoneRaw[4:8], phoneRaw[8:])
	}

	websiteURL := "https://" + tCtx.Host
	if tCtx.Tenant.CustomDomain != "" {
		websiteURL = "https://" + tCtx.Tenant.CustomDomain
	}
	websiteDisplay := strings.TrimPrefix(websiteURL, "https://")
	websiteDisplay = strings.TrimPrefix(websiteDisplay, "http://")

	email := tCtx.Tenant.ContactEmail
	if email == "" {
		email = "info@" + tCtx.Host
	}

	tenantScript := fmt.Sprintf(`
  <!-- DYNAMIC ISPSYNC TENANT INJECTION -->
  <script>
    window.__ISPSYNC_TENANT__ = {
      id: %q,
      slug: %q,
      name: %q,
      short_name: %q,
      logo_url: %q,
      brand_color: %q,
      contact_phone: %q,
      contact_email: %q,
      address: %q,
      appType: %q,
      subdomain: %q,
      host: %q,
      allTenants: %s
    };
    (function() {
      try {
        document.documentElement.setAttribute('data-subdomain', %q);
        document.title = %q;
      } catch (e) {}
    })();
  </script>
`, tCtx.Tenant.ID, tCtx.Tenant.Slug, tCtx.Tenant.Name, tCtx.Tenant.PrefixID, tCtx.Tenant.LogoURL, tCtx.Tenant.BrandColor, tCtx.Tenant.ContactPhone, tCtx.Tenant.ContactEmail, tCtx.Tenant.Address, tCtx.AppType, tCtx.Subdomain, tCtx.Host, string(allTenantsJSON), mode, pageTitle)

	// Inject script into <head>
	html = strings.Replace(html, "<head>", "<head>\n"+tenantScript, 1)

	// Inject topbar into <body> only if explicitly requested via ?demo=1 or ?switcher=1
	showSwitcher := r.URL.Query().Get("demo") == "1" || r.URL.Query().Get("switcher") == "1"
	if showSwitcher && strings.Contains(html, "<body") {
		idx := strings.Index(html, "<body")
		closeTag := strings.Index(html[idx:], ">")
		if closeTag != -1 {
			insertPos := idx + closeTag + 1
			html = html[:insertPos] + "\n" + topbarHTML + html[insertPos:]
		}
	}

	// Cleanly replace <title> tag with pre-computed server pageTitle
	if idxStart := strings.Index(html, "<title>"); idxStart != -1 {
		if idxEnd := strings.Index(html[idxStart:], "</title>"); idxEnd != -1 {
			html = html[:idxStart] + "<title>" + pageTitle + html[idxStart+idxEnd:]
		}
	}
	html = strings.Replace(html, ">GOGIGA<span class=\"text-sky-600\">NET</span><", ">"+tCtx.Tenant.PrefixID+"<span style=\"color:"+tCtx.Tenant.BrandColor+"\"> FIBER</span><", -1)
	html = strings.Replace(html, "id=\"navbar-brand-subtitle\" class=\"text-[11px] text-slate-500 font-medium mt-1\">Fiber Broadband &amp; Telco<", "id=\"navbar-brand-subtitle\" class=\"text-[11px] text-slate-500 font-medium mt-1\">"+tCtx.Tenant.Name+"<", -1)

	// Dynamic Footer & Contact replacements
	html = strings.Replace(html, "https://wa.me/6285186866164", "https://wa.me/"+phoneWA, -1)
	html = strings.Replace(html, "0851-8686-6164", phoneDisplay, -1)
	html = strings.Replace(html, "https://www.gogiga.net.id", websiteURL, -1)
	html = strings.Replace(html, "www.gogiga.net.id", websiteDisplay, -1)
	html = strings.Replace(html, "@gogiga_isp", email, -1)
	html = strings.Replace(html, "info@ispsync.id", email, -1)
	html = strings.Replace(html, "mailto:info@ispsync.id", "mailto:"+email, -1)

	// Dynamic Logo & Favicon replacements
	faviconURL := "/web/" + tCtx.Tenant.Slug + "_favicon.svg"
	html = strings.Replace(html, `<link id="dynamic-favicon" rel="icon" type="image/svg+xml" href="/web/dev_favicon.svg" />`, `<link id="dynamic-favicon" rel="icon" type="image/svg+xml" href="`+faviconURL+`" />`, 1)
	html = strings.Replace(html, `<link rel="icon" type="image/png" href="/logo.png?v=20260930" />`, `<link id="dynamic-favicon" rel="icon" type="image/svg+xml" href="`+faviconURL+`" />`, 1)
	if tCtx.Tenant.LogoURL != "" {
		html = strings.Replace(html, "/logo.png", tCtx.Tenant.LogoURL, -1)
	}

	// Dynamic Address Replacements
	tenantAddr := tCtx.Tenant.Address
	if tenantAddr == "" {
		tenantAddr = "Gedung Kantor Operasional & NOC Provider"
	}
	html = strings.Replace(html, "Depan kantor Wali, Jalan Pulutan, Koto Tuo, Kec. Harau, Kab. Lima Puluh Kota, Sumbar 26271", tenantAddr, -1)
	html = strings.Replace(html, "Depan kantor Wali, Jalan Pulutan, Koto Tuo, Kec. Harau, Kabupaten Lima Puluh Kota, Sumatera Barat 26271", tenantAddr, -1)

	// Dynamic Brand Entity Replacements
	html = strings.Replace(html, "PT GOGIGA MEDIA TEKNOLOGI", tCtx.Tenant.Name, -1)
	html = strings.Replace(html, "PT GoGiga Solusi Nusantara", tCtx.Tenant.Name, -1)
	html = strings.Replace(html, "GOGIGANET BROADBAND", brandShort+" FIBER BROADBAND", -1)
	html = strings.Replace(html, "GOGIGA ISP", brandShort+" ISP", -1)
	html = strings.Replace(html, "GOGIGANET", brandShort, -1)
	html = strings.Replace(html, "GOGIGABILL", brandShort+" BILL", -1)
	html = strings.Replace(html, "Gogiganet", brandShort, -1)
	html = strings.Replace(html, "GoGiga", brandShort, -1)
	html = strings.Replace(html, "GOGIGA", brandShort, -1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}



type PortalTemplateData struct {
	TenantName   string
	TenantPrefix string
	LogoURL      string
	BrandColor   string
	ContactPhone string
	ClientIP     string
	ClientMAC    string
	Host         string
	LoginLink    string
	DestURL      string
}

func (h *PageHandler) getClientIPMAC(r *http.Request) (string, string) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	mac := r.URL.Query().Get("mac")
	if mac == "" {
		mac = "00:00:00:00:00:00"
	}
	return ip, mac
}

func (h *PageHandler) ServeHotspot(w http.ResponseWriter, r *http.Request) {
	tCtx := middleware.GetTenantContext(r)
	if tCtx == nil || tCtx.Tenant == nil {
		http.Error(w, "Tenant not found", http.StatusNotFound)
		return
	}

	ip, mac := h.getClientIPMAC(r)
	linkLogin := r.URL.Query().Get("link-login")
	dst := r.URL.Query().Get("dst")
	if dst == "" {
		dst = r.URL.Query().Get("link-orig")
	}

	redirectURL := fmt.Sprintf("https://wifi.%s.ispsync.id/hotspot/login?ip=%s&mac=%s&link-login=%s&link-orig=%s",
		tCtx.Tenant.Slug,
		url.QueryEscape(ip),
		url.QueryEscape(mac),
		url.QueryEscape(linkLogin),
		url.QueryEscape(dst),
	)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *PageHandler) ServeIsolir(w http.ResponseWriter, r *http.Request) {
	tCtx := middleware.GetTenantContext(r)
	if tCtx == nil || tCtx.Tenant == nil {
		http.Error(w, "Tenant not found", http.StatusNotFound)
		return
	}

	ip, mac := h.getClientIPMAC(r)

	data := PortalTemplateData{
		TenantName:   tCtx.Tenant.Name,
		TenantPrefix: tCtx.Tenant.PrefixID,
		LogoURL:      tCtx.Tenant.LogoURL,
		BrandColor:   tCtx.Tenant.BrandColor,
		ContactPhone: tCtx.Tenant.ContactPhone,
		ClientIP:     ip,
		ClientMAC:    mac,
		Host:         tCtx.Host,
	}

	tmpl, err := template.ParseFiles("web/telco_isolir.html")
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}
