package main

import pluginruntime "github.com/walkmiao/flypig-plugin-sdk-go/runtime"

func main() {
	pluginruntime.Serve(NewDemoPlugin())
}
