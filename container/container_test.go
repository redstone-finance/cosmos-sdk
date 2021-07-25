package container_test

import (
	"fmt"
	"reflect"
	"testing"

	reflect2 "github.com/cosmos/cosmos-sdk/container/reflect"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/container"
)

type KVStoreKey struct {
	name string
}

type ModuleKey string

type MsgClientA struct {
	key ModuleKey
}

type KeeperA struct {
	key KVStoreKey
}

type KeeperB struct {
	key        KVStoreKey
	msgClientA MsgClientA
}

type Handler struct {
	Handle func()
}

type Command struct {
	Run func()
}

func ProvideKVStoreKey(scope container.Scope) KVStoreKey {
	return KVStoreKey{name: scope.Name()}
}

func ProvideModuleKey(scope container.Scope) (ModuleKey, error) {
	return ModuleKey(scope.Name()), nil
}

func ProvideMsgClientA(_ container.Scope, key ModuleKey) MsgClientA {
	return MsgClientA{key}
}

type ModuleA struct{}

func (ModuleA) Provide(key KVStoreKey) (KeeperA, Handler, Command) {
	return KeeperA{key}, Handler{}, Command{}
}

type ModuleB struct{}

type BDependencies struct {
	container.StructArgs

	Key KVStoreKey
	A   MsgClientA
}

type BProvides struct {
	container.StructArgs

	KeeperB  KeeperB
	Handler  Handler
	Commands []Command
}

func (ModuleB) Provide(dependencies BDependencies) BProvides {
	return BProvides{
		KeeperB: KeeperB{
			key:        dependencies.Key,
			msgClientA: dependencies.A,
		},
		Handler:  Handler{},
		Commands: []Command{{}, {}},
	}
}

func TestRun(t *testing.T) {
	require.NoError(t,
		container.Run(
			func(handlers map[string]Handler, commands []Command, a KeeperA, b KeeperB) {
				require.Len(t, handlers, 2)
				require.Equal(t, Handler{}, handlers["a"])
				require.Equal(t, Handler{}, handlers["b"])
				require.Len(t, commands, 3)
				require.Equal(t, KeeperA{
					key: KVStoreKey{name: "a"},
				}, a)
				require.Equal(t, KeeperB{
					key: KVStoreKey{name: "b"},
					msgClientA: MsgClientA{
						key: "b",
					},
				}, b)
			},
			container.Debug(),
			container.AutoGroupTypes(reflect.TypeOf(Command{})),
			container.OnePerScopeTypes(reflect.TypeOf(Handler{})),
			container.Provide(
				ProvideKVStoreKey,
				ProvideModuleKey,
				ProvideMsgClientA,
			),
			container.ProvideWithScope("a", wrapProvideMethod(ModuleA{})),
			container.ProvideWithScope("b", wrapProvideMethod(ModuleB{})),
		))
}

func wrapProvideMethod(module interface{}) reflect2.Constructor {
	method := reflect.TypeOf(module).Method(0)
	methodTy := method.Type
	var in []reflect2.Input
	var out []reflect2.Output

	for i := 1; i < methodTy.NumIn(); i++ {
		in = append(in, reflect2.Input{Type: methodTy.In(i)})
	}
	for i := 0; i < methodTy.NumOut(); i++ {
		out = append(out, reflect2.Output{Type: methodTy.Out(i)})
	}

	return reflect2.Constructor{
		In:  in,
		Out: out,
		Fn: func(values []reflect.Value) ([]reflect.Value, error) {
			values = append([]reflect.Value{reflect.ValueOf(module)}, values...)
			return method.Func.Call(values), nil
		},
		Location: reflect2.LocationFromPC(method.Func.Pointer()),
	}
}

func TestResolveError(t *testing.T) {
	require.Error(t, container.Run(
		func(x string) {},
		container.Debug(),
		container.Provide(
			func(x float64) string { return fmt.Sprintf("%f", x) },
			func(x int) float64 { return float64(x) },
			func(x float32) int { return int(x) },
		),
	))
}

func TestCyclic(t *testing.T) {
	require.Error(t, container.Run(
		func(x string) {},
		container.Provide(
			func(x int) float64 { return float64(x) },
			func(x float64) (int, string) { return int(x), "hi" },
		),
	))
}

func TestErrorOption(t *testing.T) {
	err := container.Run(func() {}, container.Error(fmt.Errorf("an error")))
	require.Error(t, err)
}
