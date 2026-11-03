package dal

import (
	"fmt"

	"golearn/wire/config"
)

// NewAdvancedDal 演示「带值参数 + cleanup + error」的 provider：
//   - dsn 由 injector 里的 wire.Value 提供；
//   - opts 由同一 Set 里的 wire.Struct 拼出来。
func NewAdvancedDal(dsn config.DSN, opts config.Options) (Dal, func(), error) {
	d := &advancedDal{dsn: dsn, opts: opts}
	return d, func() { fmt.Println("advanced dal: cleanup") }, nil
}

type advancedDal struct {
	dsn  config.DSN
	opts config.Options
}

func (d *advancedDal) Process() {
	fmt.Printf("advanced dal: dsn=%s region=%s timeout=%v debug=%v\n",
		d.dsn, d.opts.Region, d.opts.Timeout, d.opts.Debug)
}
