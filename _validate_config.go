package main
import(
	"fmt"
	config "github.com/seanocca/consensus-proxy/cmd/config"
)
func main(){
	if _,err:=config.Load("config.toml");err!=nil{panic(err)}
	fmt.Println("TOML config valid")
}
