# go-msgpack

go-msgpack is a package of MessagePack encoding for Go.

## Quick Start

```
package main

import (
	gomsgpack "github.com/nnabeyang/go-msgpack"
)

type Pair struct {
	X int
	Y int
}

func main() {
	v := Pair{X: 1, Y: 2}
	data, err := gomsgpack.Marshal(v, false)
	if err != nil {
		panic(err)
	}
	var r Pair
	if err := gomsgpack.Unmarshal(data, &r); err != nil {
		panic(err)
	}
}
```

## License

go-msgpack is published under the MIT License, see LICENSE.

## Author
[Noriaki Watanabe@nnabeyang](https://twitter.com/nnabeyang)
