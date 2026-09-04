package accesslog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/metadata"
)

var (
	userIDClaimKeys = []string{"userID", "sub", "id", "user_id"}
	nameClaimKeys   = []string{"name", "Name", "username"}
)

func buildRequestParams(c echo.Context) map[string]interface{} {
	params := make(map[string]interface{})

	if query := queryParamsFromContext(c); len(query) > 0 {
		params["query"] = sanitizeObject(query)
	}

	if pathParams := pathParamsFromContext(c); len(pathParams) > 0 {
		params["path_params"] = sanitizeObject(pathParams)
	}

	if body := requestBodyFromContext(c); len(body) > 0 {
		params["body"] = body
	}

	return params
}

func queryParamsFromContext(c echo.Context) map[string]interface{} {
	query := c.QueryParams()
	if len(query) == 0 {
		return nil
	}

	queryMap := make(map[string]interface{}, len(query))
	for key, values := range query {
		if len(values) == 1 {
			queryMap[key] = values[0]
		} else {
			queryMap[key] = values
		}
	}
	return queryMap
}

func pathParamsFromContext(c echo.Context) map[string]interface{} {
	names := c.ParamNames()
	if len(names) == 0 {
		return nil
	}

	pathParams := make(map[string]interface{}, len(names))
	for _, name := range names {
		if value := c.Param(name); value != "" {
			pathParams[name] = value
		}
	}
	if len(pathParams) == 0 {
		return nil
	}
	return pathParams
}

func captureActorFromRequest(c echo.Context) Actor {
	actor := mergeActor(Actor{}, actorFromEchoContext(c))
	actor = mergeActor(actor, actorFromClaims(claimsFromEcho(c)))

	if body := requestBodyFromContext(c); body != nil {
		if email, ok := body["email"].(string); ok && email != "" && actor.UserEmail == nil {
			actor.UserEmail = strPtr(email)
		}
		if clientKey, ok := body["client_key"].(string); ok && clientKey != "" {
			actor.ClientKey = strPtr(clientKey)
		}
		for _, key := range []string{"identifier", "identifier_hospital", "code_hospital"} {
			if code, ok := body[key].(string); ok && code != "" {
				actor.CodeHospital = strPtr(code)
				break
			}
		}
	}

	return actor
}

func enrichActorFromResponse(actor Actor, responseBody interface{}) Actor {
	if responseBody == nil {
		return actor
	}

	obj, ok := responseBody.(map[string]interface{})
	if !ok {
		return actor
	}

	data, _ := obj["data"].(map[string]interface{})
	if data == nil {
		return actor
	}

	user, _ := data["user"].(map[string]interface{})
	if user == nil {
		return actor
	}

	if actor.UserEmail == nil {
		if email, ok := user["email"].(string); ok && email != "" {
			actor.UserEmail = strPtr(email)
		}
	}
	if actor.UserName == nil {
		for _, key := range []string{"name", "nama_lengkap", "username"} {
			if name, ok := user[key].(string); ok && name != "" {
				actor.UserName = strPtr(name)
				break
			}
		}
	}
	if actor.UserUUID == nil {
		for _, key := range []string{"uuid", "uid"} {
			if uuid, ok := user[key].(string); ok && uuid != "" {
				actor.UserUUID = strPtr(uuid)
				break
			}
		}
	}
	if actor.CodeHospital == nil {
		if code, ok := user["code_hospital"].(string); ok && code != "" {
			actor.CodeHospital = strPtr(code)
		}
	}

	return actor
}

func strPtr(s string) *string {
	return &s
}

func actorFromEchoContext(c echo.Context) Actor {
	actor := Actor{}

	if id, ok := parseUserID(c.Get("userID")); ok {
		actor.UserID = &id
	}
	if email, ok := stringFromValue(c.Get("email")); ok {
		actor.UserEmail = strPtr(email)
	}
	if name, ok := stringFromValue(c.Get("name")); ok {
		actor.UserName = strPtr(name)
	}

	return actor
}

func actorFromContextKeys(ctx context.Context) Actor {
	actor := Actor{}

	if id, ok := parseUserID(ctx.Value("userID")); ok {
		actor.UserID = &id
	}
	if email, ok := stringFromValue(ctx.Value("email")); ok {
		actor.UserEmail = strPtr(email)
	}
	if name, ok := stringFromValue(ctx.Value("name")); ok {
		actor.UserName = strPtr(name)
	}

	return actor
}

func actorFromClaims(claims map[string]interface{}) Actor {
	actor := Actor{}
	if claims == nil {
		return actor
	}

	for _, key := range userIDClaimKeys {
		if v, ok := claims[key]; ok && v != nil {
			if id, ok := parseUserID(v); ok {
				actor.UserID = &id
				break
			}
		}
	}

	if email, ok := claimString(claims, "email"); ok {
		actor.UserEmail = strPtr(email)
	}

	for _, key := range nameClaimKeys {
		if name, ok := claimString(claims, key); ok {
			actor.UserName = strPtr(name)
			break
		}
	}

	return actor
}

func mergeActor(base, extra Actor) Actor {
	if base.UserID == nil && extra.UserID != nil {
		base.UserID = extra.UserID
	}
	if base.UserEmail == nil && extra.UserEmail != nil {
		base.UserEmail = extra.UserEmail
	}
	if base.UserUUID == nil && extra.UserUUID != nil {
		base.UserUUID = extra.UserUUID
	}
	if base.UserName == nil && extra.UserName != nil {
		base.UserName = extra.UserName
	}
	if base.CodeHospital == nil && extra.CodeHospital != nil {
		base.CodeHospital = extra.CodeHospital
	}
	if base.ClientKey == nil && extra.ClientKey != nil {
		base.ClientKey = extra.ClientKey
	}
	if base.TenantHubID == nil && extra.TenantHubID != nil {
		base.TenantHubID = extra.TenantHubID
	}
	return base
}

func parseUserID(val interface{}) (int, bool) {
	if val == nil {
		return 0, false
	}

	switch v := val.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		var id int
		if _, err := fmt.Sscanf(v, "%d", &id); err == nil {
			return id, true
		}
		return 0, false
	default:
		if s, ok := val.(fmt.Stringer); ok {
			var id int
			if _, err := fmt.Sscanf(s.String(), "%d", &id); err == nil {
				return id, true
			}
		}
		return 0, false
	}
}

func stringFromValue(val interface{}) (string, bool) {
	if val == nil {
		return "", false
	}
	if s, ok := val.(string); ok && s != "" {
		return s, true
	}
	return "", false
}

func claimString(claims map[string]interface{}, key string) (string, bool) {
	val, ok := claims[key]
	if !ok || val == nil {
		return "", false
	}
	if s, ok := val.(string); ok && s != "" {
		return s, true
	}
	s := fmt.Sprint(val)
	if s != "" && s != "<nil>" {
		return s, true
	}
	return "", false
}

func claimsFromEcho(c echo.Context) map[string]interface{} {
	return normalizeClaims(c.Get("claims"))
}

func claimsFromContext(ctx context.Context) map[string]interface{} {
	return normalizeClaims(ctx.Value("claims"))
}

func normalizeClaims(val interface{}) map[string]interface{} {
	if val == nil {
		return nil
	}
	if claims, ok := val.(map[string]interface{}); ok {
		return claims
	}

	data, err := jsonMarshal(val)
	if err != nil {
		return nil
	}

	var claims map[string]interface{}
	if err := jsonUnmarshal(data, &claims); err != nil {
		return nil
	}
	return claims
}

func tenantHubIDFromMetadata(md metadata.MD) string {
	if vals := md.Get("tenant-hub-id"); len(vals) > 0 && vals[0] != "" {
		return vals[0]
	}
	if vals := md.Get("x-tenant-hub-id"); len(vals) > 0 && vals[0] != "" {
		return vals[0]
	}
	return ""
}

func cacheRequestBody(c echo.Context) {
	if c.Request().Body == nil || c.Request().ContentLength == 0 {
		return
	}
	if _, ok := c.Get("_accesslog_body").([]byte); ok {
		return
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return
	}
	_ = c.Request().Body.Close()
	c.Request().Body = io.NopCloser(bytes.NewReader(body))
	c.Set("_accesslog_body", body)
}

func requestBodyFromContext(c echo.Context) map[string]interface{} {
	raw, ok := c.Get("_accesslog_body").([]byte)
	if !ok || len(raw) == 0 {
		return nil
	}

	var body map[string]interface{}
	if err := jsonUnmarshal(raw, &body); err != nil {
		return nil
	}
	return sanitizeObject(body)
}

func statusCodeFromContext(c echo.Context, err error) int {
	if err == nil {
		status := c.Response().Status
		if status == 0 {
			return http.StatusOK
		}
		return status
	}

	if he, ok := err.(*echo.HTTPError); ok {
		return he.Code
	}

	status := c.Response().Status
	if status >= 400 {
		return status
	}
	return http.StatusInternalServerError
}
