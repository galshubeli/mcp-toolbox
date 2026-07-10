// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package lookerupdatedashboardlayoutcomponent

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/googleapis/mcp-toolbox/internal/tools"
	"github.com/googleapis/mcp-toolbox/internal/util"
	"github.com/googleapis/mcp-toolbox/internal/util/parameters"

	"github.com/looker-open-source/sdk-codegen/go/rtl"
	v4 "github.com/looker-open-source/sdk-codegen/go/sdk/v4"
)

const resourceType string = "looker-update-dashboard-layout-component"

func init() {
	if !tools.Register(resourceType, newConfig) {
		panic(fmt.Sprintf("tool type %q already registered", resourceType))
	}
}

func newConfig(ctx context.Context, name string, decoder *yaml.Decoder) (tools.ToolConfig, error) {
	actual := Config{ConfigBase: tools.ConfigBase{Name: name}}
	if err := decoder.DecodeContext(ctx, &actual); err != nil {
		return nil, err
	}
	return actual, nil
}

type compatibleSource interface {
	UseClientAuthorization() bool
	GetAuthTokenHeaderName() string
	LookerApiSettings() *rtl.ApiSettings
	GetLookerSDK(context.Context, string) (*v4.LookerSDK, error)
}

type Config struct {
	tools.ConfigBase `yaml:",inline"`
	Type             string                 `yaml:"type" validate:"required"`
	Source           string                 `yaml:"source" validate:"required"`
	Annotations      *tools.ToolAnnotations `yaml:"annotations,omitempty"`
}

// validate interface
var _ tools.ToolConfig = Config{}

func (cfg Config) ToolConfigType() string {
	return resourceType
}

func (cfg Config) Initialize(context.Context) (tools.Tool, error) {
	if cfg.Description == "" {
		return nil, fmt.Errorf("description is required for tool %q", cfg.Name)
	}

	componentIdParam := parameters.NewStringParameter("dashboard_layout_component_id", "The ID of the dashboard layout component to update.")
	layoutIdParam := parameters.NewStringParameter("dashboard_layout_id", "The ID of the target dashboard layout (tab) to move the component to.", parameters.WithStringRequired(false))
	rowParam := parameters.NewIntParameter("row", "The new row position.", parameters.WithIntRequired(false))
	columnParam := parameters.NewIntParameter("column", "The new column position.", parameters.WithIntRequired(false))
	widthParam := parameters.NewIntParameter("width", "The new width.", parameters.WithIntRequired(false))
	heightParam := parameters.NewIntParameter("height", "The new height.", parameters.WithIntRequired(false))

	params := parameters.Parameters{
		componentIdParam,
		layoutIdParam,
		rowParam,
		columnParam,
		widthParam,
		heightParam,
	}

	return Tool{
		BaseTool: tools.NewBaseTool(
			cfg,
			tools.GetAnnotationsOrDefault(cfg.Annotations, tools.NewWriteAnnotations),
			tools.Manifest{Description: cfg.Description, Parameters: params.Manifest(), AuthRequired: cfg.AuthRequired},
			params,
		),
	}, nil
}

// validate interface
var _ tools.Tool = Tool{}

type Tool struct {
	tools.BaseTool[Config]
}

func (t Tool) ToConfig() tools.ToolConfig {
	return t.Cfg
}

func (t Tool) Invoke(ctx context.Context, resourceMgr tools.SourceProvider, params parameters.ParamValues, accessToken tools.AccessToken) (any, util.ToolboxError) {
	source, err := tools.GetCompatibleSource[compatibleSource](resourceMgr, t.Cfg.Source, t.Cfg.Name, t.Cfg.Type)
	if err != nil {
		return nil, util.NewClientServerError("source used is not compatible with the tool", http.StatusInternalServerError, err)
	}

	logger, err := util.LoggerFromContext(ctx)
	if err != nil {
		return nil, util.NewClientServerError("unable to get logger from ctx", http.StatusInternalServerError, err)
	}
	logger.DebugContext(ctx, "params = ", params)
	paramsMap := params.AsMap()

	componentId, ok := paramsMap["dashboard_layout_component_id"].(string)
	if !ok {
		return nil, util.NewAgentError("dashboard_layout_component_id parameter missing or invalid", nil)
	}

	wdlc := v4.WriteDashboardLayoutComponent{}

	if layoutId, ok := paramsMap["dashboard_layout_id"].(string); ok && layoutId != "" {
		wdlc.DashboardLayoutId = &layoutId
	}

	if r, ok := paramsMap["row"].(int); ok {
		rowVal := int64(r)
		wdlc.Row = &rowVal
	}
	if c, ok := paramsMap["column"].(int); ok {
		colVal := int64(c)
		wdlc.Column = &colVal
	}
	if w, ok := paramsMap["width"].(int); ok {
		widthVal := int64(w)
		wdlc.Width = &widthVal
	}
	if h, ok := paramsMap["height"].(int); ok {
		heightVal := int64(h)
		wdlc.Height = &heightVal
	}

	sdk, err := source.GetLookerSDK(ctx, string(accessToken))
	if err != nil {
		return nil, util.NewClientServerError("error getting sdk", http.StatusInternalServerError, err)
	}

	resp, err := sdk.UpdateDashboardLayoutComponent(componentId, wdlc, "", source.LookerApiSettings())
	if err != nil {
		if strings.Contains(err.Error(), "status=401") {
			return nil, util.NewClientServerError("unauthorized error", http.StatusUnauthorized, err)
		}
		return nil, util.ProcessGeneralError(err)
	}

	data := make(map[string]any)
	data["result"] = fmt.Sprintf("Dashboard layout component %s updated", componentId)
	if resp.Id != nil {
		data["id"] = *resp.Id
	}
	return data, nil
}

func (t Tool) GetAuthTokenHeaderName(resourceMgr tools.SourceProvider) (string, error) {
	source, err := tools.GetCompatibleSource[compatibleSource](resourceMgr, t.Cfg.Source, t.Cfg.Name, t.Cfg.Type)
	if err != nil {
		return "", err
	}
	return source.GetAuthTokenHeaderName(), nil
}
