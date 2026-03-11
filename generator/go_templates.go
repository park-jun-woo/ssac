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

{{- define "publish" -}}
	err {{if .FirstErr}}:={{else}}={{end}} queue.Publish(c.Request.Context(), "{{.Topic}}", map[string]any{
{{.InputFields}}
	}{{.OptionCode}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "{{.Message}}"})
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

{{- define "response_direct" -}}
	c.JSON(http.StatusOK, {{.Target}})
{{end}}

{{- define "sub_get" -}}
	{{.Result.Var}}, {{if .HasTotal}}total, {{end}}err {{if .ReAssign}}={{else}}:={{end}} {{.ModelCall}}({{.ArgsCode}})
	if err != nil {
		return fmt.Errorf("{{.Message}}: %w", err)
	}
{{end}}

{{- define "sub_post" -}}
	{{.Result.Var}}, err {{if .ReAssign}}={{else}}:={{end}} {{.ModelCall}}({{.ArgsCode}})
	if err != nil {
		return fmt.Errorf("{{.Message}}: %w", err)
	}
{{end}}

{{- define "sub_put" -}}
	err {{if .FirstErr}}:={{else}}={{end}} {{.ModelCall}}({{.ArgsCode}})
	if err != nil {
		return fmt.Errorf("{{.Message}}: %w", err)
	}
{{end}}

{{- define "sub_delete" -}}
	err {{if .FirstErr}}:={{else}}={{end}} {{.ModelCall}}({{.ArgsCode}})
	if err != nil {
		return fmt.Errorf("{{.Message}}: %w", err)
	}
{{end}}

{{- define "sub_empty" -}}
	if {{.Target}} {{.ZeroCheck}} {
		return fmt.Errorf("{{.Message}}")
	}
{{end}}

{{- define "sub_exists" -}}
	if {{.Target}} {{.ExistsCheck}} {
		return fmt.Errorf("{{.Message}}")
	}
{{end}}

{{- define "sub_state" -}}
	if err := {{.DiagramID}}state.CanTransition({{.DiagramID}}state.Input{ {{.InputFields}} }, "{{.Transition}}"); err != nil {
		return err
	}
{{end}}

{{- define "sub_auth" -}}
	if err := authz.Check(currentUser, "{{.Action}}", "{{.Resource}}", authz.Input{ {{.InputFields}} }); err != nil {
		return fmt.Errorf("{{.Message}}: %w", err)
	}
{{end}}

{{- define "sub_call_with_result" -}}
	{{.Result.Var}}, err {{if .ReAssign}}={{else}}:={{end}} {{.PkgName}}.{{.FuncMethod}}({{.PkgName}}.{{.FuncMethod}}Request{ {{.InputFields}} })
	if err != nil {
		return fmt.Errorf("{{.Message}}: %w", err)
	}
{{end}}

{{- define "sub_call_no_result" -}}
	if _, err {{if .FirstErr}}:={{else}}={{end}} {{.PkgName}}.{{.FuncMethod}}({{.PkgName}}.{{.FuncMethod}}Request{ {{.InputFields}} }); err != nil {
		return fmt.Errorf("{{.Message}}: %w", err)
	}
{{end}}

{{- define "sub_publish" -}}
	err {{if .FirstErr}}:={{else}}={{end}} queue.Publish(ctx, "{{.Topic}}", map[string]any{
{{.InputFields}}
	}{{.OptionCode}})
	if err != nil {
		return fmt.Errorf("{{.Message}}: %w", err)
	}
{{end}}
`))
