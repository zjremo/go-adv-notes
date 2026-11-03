package dal

import (
	"fmt"

	"github.com/google/wire"
)

// wire无法自己识别谁识别了什么接口，需要用户自己利用Bind来指定
var ProviderSet = wire.NewSet(NewDal, wire.Bind(new(Dal), new(*dal)))

type Dal interface {
	Process()
}

type dal struct {
}

// 故意写的生成*dal而不是Dal，用于说明上面的wire.Bind功能
func NewDal() *dal {
	return &dal{}
}

func (d *dal) Process() {
	fmt.Println("This is dal process...")
}
