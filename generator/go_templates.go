package generator

import "text/template"

var goTemplates = template.Must(template.New("").Parse(`
{{- define "currentUser" -}}
	// currentUser
	currentUser := c.MustGet("currentUser").(*model.CurrentUser)
{{end}}

{{- define "authorize" -}}
	// authorize
	allowed, err := authz.Check(currentUser, "{{.Action}}", "{{.Resource}}", {{.ID}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "권한 확인 실패"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "get" -}}
	// get
	{{.Result.Var}}, {{if .HasTotal}}total, {{end}}err := {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "guard nil" -}}
	// guard nil
	if {{.Target}} {{.ZeroCheck}} {
		c.JSON(http.StatusNotFound, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "guard exists" -}}
	// guard exists
	if {{.Target}} {{.ExistsCheck}} {
		c.JSON(http.StatusConflict, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "guard state" -}}
	// guard state
	if !{{.Target}}state.CanTransition({{.Entity}}.{{.StatusField}}, "{{.FuncName}}") {
		c.JSON(http.StatusConflict, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "post" -}}
	// post
	{{.Result.Var}}, err := {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "put" -}}
	// put
	err {{if .FirstErr}}:={{else}}={{end}} {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "delete" -}}
	// delete
	err {{if .FirstErr}}:={{else}}={{end}} {{.ModelVar}}.{{.ModelMethod}}({{.ParamArgs}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "{{.Message}}"})
		return
	}
{{end}}

{{- define "call_func" -}}
	// call func
	{{if .Result}}out{{else}}_{{end}}, err {{if .FirstErr}}:={{else}}={{end}} {{.PkgName}}.{{.FuncMethod}}({{.PkgName}}.{{.FuncMethod}}Request{ {{.InputFields}} })
	if err != nil {
		c.JSON({{.FuncErrStatus}}, gin.H{"error": "{{.Message}}"})
		return
	}
{{- if .Result}}
	{{.Result.Var}} := out.{{.ResultField}}
{{- end}}
{{end}}

{{- define "response json" -}}
	// response json
	c.JSON(http.StatusOK, gin.H{
		{{- range .Vars}}
		"{{.}}": {{.}},
		{{- end}}
		{{- if .HasTotal}}
		"total": total,
		{{- end}}
	})
{{end}}
`))
