package generator

import "text/template"

var goTemplates = template.Must(template.New("").Parse(`
{{- define "currentUser" -}}
	currentUser := c.MustGet("currentUser").(*model.CurrentUser)
{{end}}

{{- define "get" -}}
	{{.Result.Var}}, {{if .HasTotal}}total, {{end}}err {{if .ReAssign}}={{else}}:={{end}} {{.ModelCall}}({{.ArgsCode}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "post" -}}
	{{.Result.Var}}, err {{if .ReAssign}}={{else}}:={{end}} {{.ModelCall}}({{.ArgsCode}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "put" -}}
	err {{if .FirstErr}}:={{else}}={{end}} {{.ModelCall}}({{.ArgsCode}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "delete" -}}
	err {{if .FirstErr}}:={{else}}={{end}} {{.ModelCall}}({{.ArgsCode}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "empty" -}}
	if {{.Target}} {{.ZeroCheck}} {
		c.JSON(http.StatusNotFound, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "exists" -}}
	if {{.Target}} {{.ExistsCheck}} {
		c.JSON(http.StatusConflict, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "state" -}}
	if err := {{.DiagramID}}state.CanTransition({{.DiagramID}}state.Input{ {{.InputFields}} }, "{{.Transition}}"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
{{end}}

{{- define "auth" -}}
	if err := authz.Check(currentUser, "{{.Action}}", "{{.Resource}}", authz.Input{ {{.InputFields}} }); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "call_with_result" -}}
	{{.Result.Var}}, err {{if .ReAssign}}={{else}}:={{end}} {{.PkgName}}.{{.FuncMethod}}({{.PkgName}}.{{.FuncMethod}}Request{ {{.InputFields}} })
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "call_no_result" -}}
	if _, err {{if .FirstErr}}:={{else}}={{end}} {{.PkgName}}.{{.FuncMethod}}({{.PkgName}}.{{.FuncMethod}}Request{ {{.InputFields}} }); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "response" -}}
	c.JSON(http.StatusOK, gin.H{
		{{- range $key, $val := .ResponseFields}}
		"{{$key}}": {{$val}},
		{{- end}}
		{{- if .HasTotal}}
		"total": total,
		{{- end}}
	})
{{end}}
`))
