package controllers

import (
	"encoding/json"

	mi "github.com/tyzbit/go-openvpn/server/mi"
	"github.com/tyzbit/openvpn-web-ui/models"
)

//APISessionController manages vpn sessions
type APISessionController struct {
	APIBaseController
}

//KillParams contains CommonName of session to kill
type KillParams struct {
	Cname string `json:"cname"`
}

// Get lists vpn sessions
// @Title list
// @Description List vpn sessions
// @Success 200 request success
// @Failure 400 request failure
// @router / [get]
func (c *APISessionController) Get() {
	client := mi.NewClient(models.GlobalCfg.MINetwork, models.GlobalCfg.MIAddress)
	status, err := client.GetStatus()
	if err != nil {
		c.ServeJSONError(err.Error())
	} else {
		c.ServeJSONData(status)
	}
}

// Kill deletes vpn session
// @Title Kill
// @Description Delete (kill) session
// @Param    body     body     controllers.KillParams     true      "CommonName of client to kill"
// @Success 200 request success
// @Failure 400 request failure
// @router / [delete]
func (c *APISessionController) Kill() {
	client := mi.NewClient(models.GlobalCfg.MINetwork, models.GlobalCfg.MIAddress)
	p := KillParams{}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &p); err != nil {
		c.ServeJSONError(err.Error())
		return
	}

	if r, err := client.KillSession(p.Cname); err != nil {
		c.ServeJSONError(err.Error())
	} else {
		c.ServeJSONMessage(r)
	}
}
