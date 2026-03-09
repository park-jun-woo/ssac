package generator

import "text/template"

var goTemplates = template.Must(template.New("").Parse(`
{{- define "authorize" -}}
	// authorize
	allowed, err := authz.Check(currentUser, "{{.Action}}", "{{.Resource}}", {{.ID}})
	if err != nil {
		http.Error(w, "{{.Message}}", http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, "권한이 없습니다", http.StatusForbidden)
		return
	}
{{end}}

{{- define "get" -}}
	// get
	{{.Result.Var}}, {{if .HasTotal}}total, {{end}}err := {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
	if err != nil {
		http.Error(w, "{{.Message}}", http.StatusInternalServerError)
		return
	}
{{end}}

{{- define "guard nil" -}}
	// guard nil
	if {{.Target}} {{.ZeroCheck}} {
		http.Error(w, "{{.Message}}", http.StatusNotFound)
		return
	}
{{end}}

{{- define "guard exists" -}}
	// guard exists
	if {{.Target}} {{.ExistsCheck}} {
		http.Error(w, "{{.Message}}", http.StatusConflict)
		return
	}
{{end}}

{{- define "guard state" -}}
	// guard state
	if !{{.Target}}state.CanTransition({{.Entity}}.{{.StatusField}}, "{{.FuncName}}") {
		http.Error(w, "{{.Message}}", http.StatusConflict)
		return
	}
{{end}}

{{- define "post" -}}
	// post
	{{.Result.Var}}, err := {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
	if err != nil {
		http.Error(w, "{{.Message}}", http.StatusInternalServerError)
		return
	}
{{end}}

{{- define "put" -}}
	// put
	err {{if .FirstErr}}:={{else}}={{end}} {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
	if err != nil {
		http.Error(w, "{{.Message}}", http.StatusInternalServerError)
		return
	}
{{end}}

{{- define "delete" -}}
	// delete
	err {{if .FirstErr}}:={{else}}={{end}} {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
	if err != nil {
		http.Error(w, "{{.Message}}", http.StatusInternalServerError)
		return
	}
{{end}}

{{- define "password" -}}
	// password
	if err := bcrypt.CompareHashAndPassword([]byte({{.Hash}}), []byte({{.Plain}})); err != nil {
		http.Error(w, "{{.Message}}", http.StatusUnauthorized)
		return
	}
{{end}}

{{- define "call_component" -}}
	// call component
	{{if .Result}}{{.Result.Var}}, {{end}}err {{if .FirstErr}}:={{else}}={{end}} {{.Component}}.{{.ComponentMethod}}({{.ParamArgs}})
	if err != nil {
		http.Error(w, "{{.Message}}", http.StatusInternalServerError)
		return
	}
{{end}}

{{- define "call_func" -}}
	// call func
	{{if .Result}}{{.Result.Var}}, {{end}}err {{if .FirstErr}}:={{else}}={{end}} {{.Func}}({{.ParamArgs}})
	if err != nil {
		http.Error(w, "{{.Message}}", http.StatusInternalServerError)
		return
	}
{{end}}

{{- define "response json" -}}
	// response json
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		{{- range .Vars}}
		"{{.}}": {{.}},
		{{- end}}
		{{- if .HasTotal}}
		"total": total,
		{{- end}}
	})
{{end}}
`))
