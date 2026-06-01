package main


import (
	"html/template"
	"net/http"
	"strings"
)

func wantsHTML(req *http.Request) bool {
	a := req.Header.Get("Accept")
	return strings.Contains(a, "text/html") && !strings.Contains(a, "application/json")
}

var indexTmpl = template.Must(template.New("index").Parse(`<!doctype html>
<title>pastes</title>
<style>body{font:14px monospace;max-width:48rem;margin:2rem auto;padding:0 1rem}
a{text-decoration:none;color:#06c}a:hover{text-decoration:underline}
form{display:flex;gap:.5rem;margin:1rem 0}input{flex:1;padding:.4rem;font:inherit}
button{padding:.4rem .8rem;font:inherit;cursor:pointer}
li{list-style:none;padding:.2rem 0}.p{color:#666;margin-left:.5rem}</style>
<h1>pastes</h1>
<form onsubmit="event.preventDefault();location='/'+this.id.value">
  <input name=id placeholder="new paste id (or leave blank for auto)" autofocus>
  <button>open</button>
</form>
<ul>
{{range .}}<li><a href="/{{.ID}}">{{.ID}}</a><span class=p>{{.Preview}}</span></li>
{{end}}</ul>
`))

var viewTmpl = template.Must(template.New("view").Parse(`<!doctype html>
<title>{{.ID}}</title>
<style>body{font:14px monospace;max-width:60rem;margin:1rem auto;padding:0 1rem}
header{display:flex;justify-content:space-between;align-items:center;margin-bottom:.5rem}
a{color:#06c;text-decoration:none}a:hover{text-decoration:underline}
textarea{width:100%;height:70vh;font:inherit;padding:.5rem;box-sizing:border-box}
button{padding:.4rem 1rem;font:inherit;cursor:pointer}
.row{display:flex;gap:.5rem;align-items:center;margin-top:.5rem}
.status{color:#666}</style>
<header>
  <h2>{{.ID}}{{if .Version}} <small>@ {{.Version}}</small>{{end}}</h2>
  <nav><a href="/">all</a> · <a href="/{{.ID}}/history">history</a></nav>
</header>
<textarea id=t>{{.Content}}</textarea>
<div class=row>
  <button id=save>save</button>
  <button id=del>delete</button>
  <span id=status class=status></span>
</div>
<script>
const id={{.ID}};
const status=document.getElementById('status');
document.getElementById('save').onclick=async()=>{
  status.textContent='saving…';
  const r=await fetch('/'+id,{method:'POST',body:document.getElementById('t').value});
  status.textContent=r.ok?('saved '+(r.headers.get('X-Commit')||'').slice(0,8)):'error';
};
document.getElementById('del').onclick=async()=>{
  if(!confirm('delete '+id+'?'))return;
  const r=await fetch('/'+id,{method:'DELETE'});
  if(r.ok)location='/';else status.textContent='error';
};
</script>
`))

var historyTmpl = template.Must(template.New("history").Parse(`<!doctype html>
<title>{{.ID}} history</title>
<style>body{font:14px monospace;max-width:60rem;margin:1rem auto;padding:0 1rem}
a{color:#06c;text-decoration:none}a:hover{text-decoration:underline}
table{border-collapse:collapse;width:100%}td{padding:.2rem .5rem;border-bottom:1px solid #eee}
.t{color:#666;white-space:nowrap}</style>
<h2><a href="/{{.ID}}">{{.ID}}</a> history · <a href="/">all</a></h2>
<table>
{{range .Entries}}<tr>
  <td><a href="/{{$.ID}}?version={{.SHA}}">{{.Short}}</a></td>
  <td class=t>{{.When}}</td>
  <td>{{.Preview}}</td>
</tr>{{end}}</table>
`))

