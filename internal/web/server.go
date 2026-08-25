package web

import (
	"encoding/json"
	"html/template"
	"net/http"

	"imagerelayworker/internal/config"
)

type Server struct {
	Config  config.Config
	Running func() bool
	Addr    string
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "workerRunning": s.Running()})
	})
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			c := s.Config
			c.WorkerToken = ""
			json.NewEncoder(w).Encode(c)
			return
		}
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", 405)
			return
		}
		var c config.Config
		if json.NewDecoder(r.Body).Decode(&c) != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		if c.WorkerToken == "" {
			if old, err := config.LoadFile(); err == nil {
				c.WorkerToken = old.WorkerToken
			}
		}
		if err := config.SaveFile(c); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		s.Config = c
		json.NewEncoder(w).Encode(map[string]any{"saved": true, "restartRequired": true})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		template.Must(template.New("page").Parse(page)).Execute(w, s)
	})
	return mux
}

func (s *Server) Listen() error { return http.ListenAndServe(s.Addr, s.Handler()) }

var page = `<!doctype html><html><head><meta charset="utf-8"><title>ImageRelayWorker</title><style>body{font:16px system-ui;max-width:680px;margin:40px auto;padding:0 20px}input{display:block;width:100%;padding:10px;margin:6px 0 16px;box-sizing:border-box}button{padding:10px 18px}#status{margin:18px 0;padding:12px;background:#eee}</style></head><body><h1>ImageRelayWorker</h1><div id="status">Cargando estado...</div><form><label>API URL<input id="api" required></label><label>Token<input id="token" type="password" placeholder="No se muestra por seguridad"></label><label>Worker ID<input id="id" required></label><button>Guardar configuración</button></form><script>
const $=id=>document.getElementById(id); async function load(){let h=await fetch('/api/health').then(r=>r.json());$('status').textContent=h.ok?'Web activa · Worker '+(h.workerRunning?'ejecutándose':'detenido'): 'Error';let c=await fetch('/api/config').then(r=>r.json());$('api').value=c.APIBaseURL||'';$('id').value=c.WorkerID||''} document.querySelector('form').onsubmit=async e=>{e.preventDefault();let c={APIBaseURL:$('api').value,WorkerToken:$('token').value,WorkerID:$('id').value};let r=await fetch('/api/config',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(c)});alert(r.ok?'Guardado. Reiniciá el worker para aplicar cambios.':await r.text())};load();setInterval(load,5000);
</script></body></html>`
