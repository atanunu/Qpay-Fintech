package httpapi
import ("testing";"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service")
func TestOpenAPIParametersAreUniqueAndKeepConcreteSchemas(t *testing.T) {
  h:=New(&service.Service{},Config{});paths:=h.OpenAPI()["paths"].(map[string]any)
  for path,methods:=range paths {for method,value:=range methods.(map[string]any) {
    op:=value.(map[string]any);params,_:=op["parameters"].([]any);seen:=map[string]bool{}
    for _,value:=range params {p:=value.(map[string]any);key:=p["in"].(string)+":"+p["name"].(string)
      if seen[key]{t.Fatalf("%s %s duplicates %s",method,path,key)};seen[key]=true
      if path=="/v1/statements"&&(p["name"]=="from"||p["name"]=="to") {if p["required"]!=true||p["schema"].(map[string]any)["format"]!="date-time"{t.Fatal("statement date constraints lost")}}
      if path=="/v1/payments"&&p["name"]=="limit"&&p["schema"].(map[string]any)["type"]!="integer"{t.Fatal("pagination integer schema lost")}
    }
  }}
}
