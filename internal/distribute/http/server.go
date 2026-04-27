package http

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hotnew/internal/app"
	"hotnew/internal/config"
	"hotnew/internal/distribute"
	"hotnew/internal/domain"
)

type Server struct {
	cfg           config.Config
	runner        *app.Runner
	retry         *app.RetryProcessor
	store         domain.ArticleStore
	deliveryStore domain.DeliveryStore
	retryQueue    domain.RetryQueue
	registry      domain.SourceRegistry
}

func NewServer(cfg config.Config, runner *app.Runner, retry *app.RetryProcessor, store domain.ArticleStore, deliveryStore domain.DeliveryStore, retryQueue domain.RetryQueue, registry domain.SourceRegistry) Server {
	return Server{cfg: cfg, runner: runner, retry: retry, store: store, deliveryStore: deliveryStore, retryQueue: retryQueue, registry: registry}
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/articles", s.handleArticles)
	mux.HandleFunc("/v1/articles/get", s.handleArticleGet)
	mux.HandleFunc("/v1/blog/preview", s.handleBlogPreview)
	mux.HandleFunc("/v1/deliveries", s.handleDeliveries)
	mux.HandleFunc("/v1/retries", s.handleRetries)
	mux.HandleFunc("/v1/retries/run", s.handleRetryRun)
	mux.HandleFunc("/v1/retries/run-one", s.handleRetryRunOne)
	mux.HandleFunc("/v1/retries/reset", s.handleRetryReset)
	mux.HandleFunc("/v1/retries/archive", s.handleRetryArchive)
	mux.HandleFunc("/v1/sources", s.handleSources)
	mux.HandleFunc("/v1/run", s.handleRun)
	mux.HandleFunc("/v1/status", s.handleStatus)
	return mux
}

func (s Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = dashboardTemplate.Execute(w, map[string]any{"title": "hotnew dashboard"})
}

func (s Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
func (s Server) handleArticles(w http.ResponseWriter, r *http.Request) {
	limit := parseInt(r.URL.Query().Get("limit"), 20)
	items, err := s.store.List(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s Server) handleArticleGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	article, ok, err := s.store.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
		return
	}
	writeJSON(w, http.StatusOK, article)
}

func (s Server) handleBlogPreview(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	article, ok, err := s.store.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
		return
	}
	blogDistributor, err := distribute.NewBlogDistributor(s.cfg.Distribute.Blog)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, blogDistributor.BuildPost(article))
}

func (s Server) handleDeliveries(w http.ResponseWriter, r *http.Request) {
	if s.deliveryStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []domain.DeliveryRecord{}})
		return
	}
	limit := parseInt(r.URL.Query().Get("limit"), 20)
	items, err := s.deliveryStore.List(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s Server) handleRetries(w http.ResponseWriter, r *http.Request) {
	if s.retryQueue == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []domain.RetryJob{}})
		return
	}
	filter := domain.RetryFilter{Status: strings.TrimSpace(r.URL.Query().Get("status")), Channel: strings.TrimSpace(r.URL.Query().Get("channel")), ArticleID: strings.TrimSpace(r.URL.Query().Get("article_id")), Limit: parseInt(r.URL.Query().Get("limit"), 20)}
	items, err := s.retryQueue.ListFiltered(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s Server) handleRetryRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.retry == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "retry processor not configured"})
		return
	}
	limit := parseInt(r.URL.Query().Get("limit"), s.cfg.Retry.BatchSize)
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.Retry.Timeout)
	defer cancel()
	result, err := s.retry.ProcessOnce(ctx, limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s Server) handleRetryRunOne(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.retry == nil || s.retryQueue == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "retry processor not configured"})
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.Retry.Timeout)
	defer cancel()
	result, ok, err := s.retry.ProcessJob(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "retry job not found or already processing"})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s Server) handleRetryReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.retryQueue == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "retry queue not configured"})
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	ok, err := s.retryQueue.Reset(r.Context(), id, time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "retry job not found"})
		return
	}
	job, _, err := s.retryQueue.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s Server) handleRetryArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.retryQueue == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "retry queue not configured"})
		return
	}
	duration := s.cfg.Retry.ArchiveAfter
	if raw := strings.TrimSpace(r.URL.Query().Get("older_than")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid older_than duration"})
			return
		}
		duration = parsed
	}
	limit := parseInt(r.URL.Query().Get("limit"), s.cfg.Retry.ArchiveBatch)
	archived, err := s.retryQueue.ArchiveSucceededBefore(r.Context(), time.Now().UTC().Add(-duration), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"archived": archived, "older_than": duration.String()})
}

func (s Server) handleSources(w http.ResponseWriter, r *http.Request) {
	items, err := s.registry.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	limit := parseInt(r.URL.Query().Get("limit"), 10)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := s.runner.RunNow(ctx, limit, "manual")
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, app.ErrRunInProgress) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.runner.Status())
}
func parseInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

var dashboardTemplate = template.Must(template.New("dashboard").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.title}}</title>
<style>
:root{--bg:#f3efe5;--card:#fffdf7;--ink:#1e2a2f;--muted:#68757b;--accent:#0f766e;--line:#d8d2c4;--warn:#b45309;--bad:#b91c1c}
*{box-sizing:border-box} body{margin:0;font-family:Georgia,"Noto Serif",serif;background:linear-gradient(180deg,#efe7d7 0%,#f7f4ec 45%,#f3efe5 100%);color:var(--ink)}
header{padding:32px 24px 16px;border-bottom:1px solid rgba(30,42,47,.08);background:radial-gradient(circle at top left,#fff7d6 0,#f3efe5 50%)}
header h1{margin:0;font-size:32px;letter-spacing:.02em} header p{margin:10px 0 0;color:var(--muted);max-width:760px}
main{padding:20px;display:grid;gap:18px} .grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:18px}
.card{background:var(--card);border:1px solid var(--line);border-radius:18px;box-shadow:0 10px 30px rgba(30,42,47,.05);overflow:hidden}
.card h2{margin:0;padding:16px 18px;border-bottom:1px solid var(--line);font-size:18px;background:rgba(255,255,255,.55)}
.section{padding:16px 18px} .toolbar{display:flex;flex-wrap:wrap;gap:10px;align-items:center;margin-bottom:12px}
input,select,button{font:inherit;border-radius:10px;border:1px solid var(--line);padding:9px 12px;background:#fff}
button{cursor:pointer;background:var(--accent);border-color:var(--accent);color:#fff} button.secondary{background:#fff;color:var(--ink)} button.warn{background:var(--warn);border-color:var(--warn)}
.stats{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.stat{padding:14px;border:1px solid var(--line);border-radius:14px;background:#fff}.stat b{display:block;font-size:22px}.stat span{color:var(--muted);font-size:13px}
.table-wrap{overflow:auto;max-height:420px}.table{width:100%;border-collapse:collapse;font-size:14px}.table th,.table td{padding:10px 12px;border-bottom:1px solid var(--line);vertical-align:top;text-align:left}.table th{position:sticky;top:0;background:#fbf8ef;z-index:1}.muted{color:var(--muted)} .status-badge{display:inline-block;padding:3px 8px;border-radius:999px;font-size:12px;border:1px solid currentColor}.status-failed{color:var(--bad)}.status-succeeded{color:var(--accent)}.status-retrying,.status-queued,.status-processing{color:var(--warn)}
pre{margin:0;white-space:pre-wrap;word-break:break-word}.actions{display:flex;gap:8px;flex-wrap:wrap}.foot{padding:0 20px 24px;color:var(--muted);font-size:13px}
@media (max-width:720px){.stats{grid-template-columns:repeat(2,minmax(0,1fr))} header h1{font-size:28px}}
</style>
</head>
<body>
<header><h1>hotnew dashboard</h1><p>Read-only console for articles, deliveries, retries, and runtime status. Retry operations are available for failed jobs and use the same backend APIs as automation.</p></header>
<main>
<section class="card"><div class="section"><div class="stats"><div class="stat"><b id="stat-articles">0</b><span>Articles</span></div><div class="stat"><b id="stat-deliveries">0</b><span>Deliveries</span></div><div class="stat"><b id="stat-retries">0</b><span>Retry Jobs</span></div><div class="stat"><b id="stat-running">idle</b><span>Runner</span></div></div></div></section>
<section class="grid">
<div class="card"><h2>Articles</h2><div class="section"><div class="toolbar"><button id="refresh-all">Refresh</button><button id="run-now" class="secondary">Run Ingest</button></div><div class="table-wrap"><table class="table" id="articles-table"><thead><tr><th>Title</th><th>Source</th><th>Published</th><th>ID</th></tr></thead><tbody></tbody></table></div></div></div>
<div class="card"><h2>Deliveries</h2><div class="section"><div class="toolbar"><button id="refresh-deliveries" class="secondary">Refresh</button></div><div class="table-wrap"><table class="table" id="deliveries-table"><thead><tr><th>Channel</th><th>Status</th><th>Article</th><th>When</th><th>Error</th></tr></thead><tbody></tbody></table></div></div></div>
</section>
<section class="card"><h2>Retries</h2><div class="section"><div class="toolbar"><select id="retry-status"><option value="">All Status</option><option value="queued">queued</option><option value="retrying">retrying</option><option value="processing">processing</option><option value="failed">failed</option><option value="succeeded">succeeded</option></select><input id="retry-channel" placeholder="channel"><input id="retry-article" placeholder="article_id"><button id="filter-retries">Filter</button><button id="archive-retries" class="warn">Archive Old Succeeded</button></div><div class="table-wrap"><table class="table" id="retries-table"><thead><tr><th>ID</th><th>Status</th><th>Channel</th><th>Article</th><th>Attempts</th><th>Next Attempt</th><th>Actions</th></tr></thead><tbody></tbody></table></div></div></section>
<section class="card"><h2>Status</h2><div class="section"><pre id="status-box">loading...</pre></div></section>
</main>
<div class="foot">Dashboard uses existing API endpoints only. No frontend build step required.</div>
<script>
const $ = (s) => document.querySelector(s);
const fmt = (v) => v ? new Date(v).toLocaleString() : '-';
const esc = (v) => String(v ?? '').replace(/[&<>"']/g, function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]; });
async function jsonFetch(url, opts){ const res = await fetch(url, opts); const data = await res.json().catch(function(){ return {}; }); if(!res.ok) throw new Error(data.error || res.statusText); return data; }
async function loadStatus(){ const data = await jsonFetch('/v1/status'); $('#status-box').textContent = JSON.stringify(data, null, 2); $('#stat-running').textContent = data.running ? 'running' : 'idle'; }
async function loadArticles(){ const data = await jsonFetch('/v1/articles?limit=20'); $('#stat-articles').textContent = data.items.length; $('#articles-table tbody').innerHTML = data.items.map(function(a){ return '<tr><td><a href="'+esc(a.url)+'" target="_blank" rel="noreferrer">'+esc(a.title)+'</a></td><td>'+esc(a.source)+'</td><td>'+fmt(a.published_at)+'</td><td class="muted">'+esc(a.id)+'</td></tr>'; }).join(''); }
async function loadDeliveries(){ const data = await jsonFetch('/v1/deliveries?limit=20'); $('#stat-deliveries').textContent = data.items.length; $('#deliveries-table tbody').innerHTML = data.items.map(function(d){ return '<tr><td>'+esc(d.channel)+'</td><td><span class="status-badge status-'+esc(d.status)+'">'+esc(d.status)+'</span></td><td class="muted">'+esc(d.article_id)+'</td><td>'+fmt(d.attempted_at)+'</td><td class="muted">'+esc(d.error || '')+'</td></tr>'; }).join(''); }
async function loadRetries(){ const p = new URLSearchParams(); p.set('limit','50'); if($('#retry-status').value) p.set('status',$('#retry-status').value); if($('#retry-channel').value) p.set('channel',$('#retry-channel').value); if($('#retry-article').value) p.set('article_id',$('#retry-article').value); const data = await jsonFetch('/v1/retries?'+p.toString()); $('#stat-retries').textContent = data.items.length; $('#retries-table tbody').innerHTML = data.items.map(function(j){ return '<tr><td class="muted">'+esc(j.id)+'</td><td><span class="status-badge status-'+esc(j.status)+'">'+esc(j.status)+'</span></td><td>'+esc(j.channel)+'</td><td class="muted">'+esc(j.article_id)+'</td><td>'+esc(j.attempts)+'/'+esc(j.max_attempts)+'</td><td>'+fmt(j.next_attempt_at)+'</td><td><div class="actions"><button data-run="'+esc(j.id)+'">Run</button><button data-reset="'+esc(j.id)+'" class="secondary">Reset</button></div></td></tr>'; }).join(''); bindRetryActions(); }
function bindRetryActions(){ document.querySelectorAll('[data-run]').forEach(function(btn){ btn.onclick = async function(){ try { await jsonFetch('/v1/retries/run-one?id='+encodeURIComponent(btn.dataset.run), {method:'POST'}); await refreshAll(); } catch (e) { alert(e.message); } }; }); document.querySelectorAll('[data-reset]').forEach(function(btn){ btn.onclick = async function(){ try { await jsonFetch('/v1/retries/reset?id='+encodeURIComponent(btn.dataset.reset), {method:'POST'}); await loadRetries(); } catch (e) { alert(e.message); } }; }); }
async function refreshAll(){ try { await Promise.all([loadStatus(), loadArticles(), loadDeliveries(), loadRetries()]); } catch (e) { alert(e.message); } }
$('#refresh-all').onclick = refreshAll; $('#refresh-deliveries').onclick = loadDeliveries; $('#filter-retries').onclick = loadRetries; $('#run-now').onclick = async function(){ try { await jsonFetch('/v1/run?limit=10', {method:'POST'}); await refreshAll(); } catch (e) { alert(e.message); } }; $('#archive-retries').onclick = async function(){ try { await jsonFetch('/v1/retries/archive', {method:'POST'}); await loadRetries(); } catch (e) { alert(e.message); } };
refreshAll();
</script>
</body></html>`))
