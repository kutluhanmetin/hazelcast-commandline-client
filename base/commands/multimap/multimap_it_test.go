package _multimap_test

import (
	"context"
	"testing"

	_ "github.com/hazelcast/hazelcast-commandline-client/base/commands"
	"github.com/hazelcast/hazelcast-commandline-client/internal/check"
	"github.com/hazelcast/hazelcast-commandline-client/internal/it"
	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/types"
	"github.com/stretchr/testify/require"
)

func TestMultimap(t *testing.T) {
	testCases := []struct {
		name string
		f    func(t *testing.T)
	}{
		{name: "Put_NonInteractive", f: put_NonInteractiveTest},
		{name: "Get_Noninteractive", f: get_NonInteractiveTest},
		{name: "Remove_Noninteractive", f: remove_NonInteractiveTest},
		{name: "Size_Noninteractive", f: size_NoninteractiveTest},
		{name: "Clear_NonInteractive", f: clear_NonInteractiveTest},
		{name: "Destroy_NonInteractive", f: destroy_NonInteractiveTest},
		{name: "KeySet_NoninteractiveTest", f: keySet_NoninteractiveTest},
		{name: "EntrySet_NonInteractive", f: entrySet_NonInteractiveTest},
		{name: "EntrySet_NonInteractive", f: entrySet_NonInteractiveTest},
		{name: "Values_NonInteractive", f: values_NonInteractiveTest},
	}
	for _, tc := range testCases {
		t.Run(tc.name, tc.f)
	}
}

func put_NonInteractiveTest(t *testing.T) {
	it.MultiMapTester(t, func(tcx it.TestContext, m *hazelcast.MultiMap) {
		t := tcx.T
		ctx := context.Background()
		tcx.WithReset(func() {
			tcx.CLCExecute(ctx, "multimap", "-n", m.Name(), "put", "foo", "bar", "-q")
			tcx.CLCExecute(ctx, "multimap", "-n", m.Name(), "put", "foo", "bar2", "-q")
			tcx.AssertStderrEquals("")
			v := check.MustValue(m.Get(context.Background(), "foo"))
			require.Contains(t, v, "bar")
			require.Contains(t, v, "bar2")
		})
	})
}

func get_NonInteractiveTest(t *testing.T) {
	it.MultiMapTester(t, func(tcx it.TestContext, m *hazelcast.MultiMap) {
		ctx := context.Background()
		// no entry
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "get", "foo", "-q"))
			tcx.AssertStdoutEquals("")
		})
		// set an entry
		tcx.WithReset(func() {
			check.MustValue(m.Put(context.Background(), "foo", "bar"))
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "get", "foo", "-q", "--show-type"))
			tcx.AssertStdoutEquals("bar\tSTRING\n")
		})
	})
}

func remove_NonInteractiveTest(t *testing.T) {
	it.MultiMapTester(t, func(tcx it.TestContext, m *hazelcast.MultiMap) {
		ctx := context.Background()
		tcx.WithReset(func() {
			check.MustValue(m.Put(ctx, "foo", "bar"))
			size := check.MustValue(m.Size(ctx))
			require.Equal(tcx.T, 1, size)
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "remove", "foo", "-q", "--show-type"))
			tcx.AssertStdoutEquals("bar\tSTRING\n")
			size = check.MustValue(m.Size(ctx))
			require.Equal(tcx.T, 0, size)
		})
	})
}

func size_NoninteractiveTest(t *testing.T) {
	it.MultiMapTester(t, func(tcx it.TestContext, m *hazelcast.MultiMap) {
		ctx := context.Background()
		// no entry
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "size", "-q"))
			tcx.AssertStdoutEquals("0\n")
		})
		// set an entry
		tcx.WithReset(func() {
			check.MustValue(m.Put(ctx, "foo", "bar"))
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "size", "-q"))
			tcx.AssertStdoutEquals("1\n")
		})
	})
}

func clear_NonInteractiveTest(t *testing.T) {
	it.MultiMapTester(t, func(tcx it.TestContext, m *hazelcast.MultiMap) {
		t := tcx.T
		ctx := context.Background()
		tcx.WithReset(func() {
			check.MustValue(m.Put(ctx, "foo", "bar"))
			require.Equal(t, 1, check.MustValue(m.Size(ctx)))
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "clear", "-q", "--yes"))
			require.Equal(t, 0, check.MustValue(m.Size(ctx)))
		})
	})
}

func destroy_NonInteractiveTest(t *testing.T) {
	it.MultiMapTester(t, func(tcx it.TestContext, m *hazelcast.MultiMap) {
		t := tcx.T
		ctx := context.Background()
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "destroy", "--yes"))
			objects := check.MustValue(tcx.Client.GetDistributedObjectsInfo(ctx))
			require.False(t, objectExists(hazelcast.ServiceNameMap, m.Name(), objects))
		})
	})
}

func keySet_NoninteractiveTest(t *testing.T) {
	it.MultiMapTester(t, func(tcx it.TestContext, m *hazelcast.MultiMap) {
		ctx := context.Background()
		// no entry
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "key-set", "-q"))
			tcx.AssertStdoutEquals("")
		})
		// set an entry
		tcx.WithReset(func() {
			check.MustValue(m.Put(context.Background(), "foo", "bar"))
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "key-set", "-q"))
			tcx.AssertStdoutContains("foo\n")
		})
		// show type
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "key-set", "--show-type", "-q"))
			tcx.AssertStdoutContains("foo\tSTRING\n")
		})
	})
}

func entrySet_NonInteractiveTest(t *testing.T) {
	it.MultiMapTester(t, func(tcx it.TestContext, m *hazelcast.MultiMap) {
		ctx := context.Background()
		// no entry
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "entry-set", "-q"))
			tcx.AssertStdoutEquals("")
		})
		// set an entry
		tcx.WithReset(func() {
			check.MustValue(m.Put(context.Background(), "foo", "bar"))
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "entry-set", "-q"))
			tcx.AssertStdoutContains("foo\tbar\n")
		})
		// show type
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "entry-set", "--show-type", "-q"))
			tcx.AssertStdoutContains("foo\tSTRING\tbar\tSTRING\n")
		})
	})
}

func values_NonInteractiveTest(t *testing.T) {
	it.MultiMapTester(t, func(tcx it.TestContext, m *hazelcast.MultiMap) {
		ctx := context.Background()
		// no entry
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "values", "-q"))
			tcx.AssertStdoutEquals("")
		})
		// set an entry
		tcx.WithReset(func() {
			check.MustValue(m.Put(context.Background(), "foo", "bar"))
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "values", "-q"))
			tcx.AssertStdoutContains("bar\n")
		})
		// show type
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute(ctx, "multimap", "-n", m.Name(), "values", "--show-type", "-q"))
			tcx.AssertStdoutContains("bar\tSTRING\n")
		})
	})
}

func objectExists(sn, name string, objects []types.DistributedObjectInfo) bool {
	for _, obj := range objects {
		if sn == obj.ServiceName && name == obj.Name {
			return true
		}
	}
	return false
}