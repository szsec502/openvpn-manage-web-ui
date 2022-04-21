package routers

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context/param"
)

func init() {

    beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:APISessionController"] = append(beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:APISessionController"],
        beego.ControllerComments{
            Method: "Get",
            Router: "/",
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:APISessionController"] = append(beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:APISessionController"],
        beego.ControllerComments{
            Method: "Kill",
            Router: "/",
            AllowHTTPMethods: []string{"delete"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:APISignalController"] = append(beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:APISignalController"],
        beego.ControllerComments{
            Method: "Send",
            Router: "/",
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:APISysloadController"] = append(beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:APISysloadController"],
        beego.ControllerComments{
            Method: "Get",
            Router: "/",
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:CertificatesController"] = append(beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:CertificatesController"],
        beego.ControllerComments{
            Method: "Get",
            Router: "/certificates",
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:CertificatesController"] = append(beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:CertificatesController"],
        beego.ControllerComments{
            Method: "Post",
            Router: "/certificates",
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:CertificatesController"] = append(beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:CertificatesController"],
        beego.ControllerComments{
            Method: "Download",
            Router: "/certificates/:key",
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:CertificatesController"] = append(beego.GlobalControllerRouter["github.com/bnhf/openvpn-tap-external-web-ui/controllers:CertificatesController"],
        beego.ControllerComments{
            Method: "DownloadSingleConfig",
            Router: "/certificates/single-config/:key",
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

}
