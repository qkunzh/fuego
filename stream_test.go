package fuego

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// func functionTimesTwo() Function {
// 	return func(i EntryInt) EntryInt {
// 		return EntryInt(2 * i)
// 	}
// }

func functionSlowTimesTwo() Function[EntryInt] {
	return func(i EntryInt) EntryInt {
		time.Sleep(50 * time.Millisecond)
		return EntryInt(2 * i)
	}
}

func entryIntEqualsTo(number EntryInt) Function {
	return func(subject EntryInt) Entry[bool] {
		return EntryBool(number.Equal(subject))
	}
}

func TestStream_Map(t *testing.T) {
	type fields struct {
		stream chan Entry
	}
	type args struct {
		mapper Function
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntrySlice
	}{
		{
			name:   "Should return an empty Stream",
			fields: fields{stream: nil},
			args: args{
				mapper: functionTimesTwo(),
			},
			want: EntrySlice{},
		},
		{
			name: "Should return a Stream of doubled integers",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry, 1)
					go func() {
						defer close(c)
						c <- EntryInt(1)
						c <- EntryInt(3)
						c <- EntryInt(2)
					}()
					return c
				}()},
			args: args{
				mapper: functionTimesTwo(),
			},
			want: EntrySlice{
				EntryInt(2),
				EntryInt(6),
				EntryInt(4)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}

			var got EntrySlice
			stream := s.Map(tt.args.mapper)
			if gotStream := stream.stream; gotStream != nil {
				got = EntrySlice{}
				for val := range gotStream {
					got = append(got, val)
				}
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Stream.Map() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStream_Map_Concurrent(t *testing.T) {
	const numEntries = 300
	const concurrencyLevel = numEntries / 10

	sourceStream := func() chan Entry {
		c := make(chan Entry, 10)
		go func() {
			defer close(c)
			for i := 0; i < numEntries; i++ {
				c <- EntryInt(i)
			}
		}()
		return c
	}()

	want := func() EntrySlice {
		es := EntrySlice{}
		for i := 0; i < numEntries; i++ {
			es = es.Append(functionTimesTwo()(EntryInt(i)))
		}
		return es
	}()

	unitStream := Stream{
		stream: sourceStream,
	}

	// functionSlowTimesTwo: use slow function to illustrate the performance improvement
	gotStream := unitStream.Concurrent(concurrencyLevel).Map(functionSlowTimesTwo()).ToSlice()

	if !reflect.DeepEqual(gotStream, want) {
		t.Errorf("Stream.Map() = %v, want %v", gotStream, want)
	}
}

func TestStream_Filter(t *testing.T) {
	type fields struct {
		stream chan Entry
	}
	type args struct {
		predicate Predicate
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntrySlice
	}{
		{
			name:   "Should return nil for an empty Stream",
			fields: fields{stream: nil},
			args: args{
				predicate: intGreaterThanPredicate(5),
			},
			want: EntrySlice{},
		},
		{
			name: "Should give produce filtered values as per predicate",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry, 1)
					go func() {
						defer close(c)
						c <- EntryInt(17)
						c <- EntryInt(8)
						c <- EntryInt(2)
					}()
					return c
				}()},
			args: args{
				predicate: intGreaterThanPredicate(5),
			},
			want: EntrySlice{
				EntryInt(17),
				EntryInt(8)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}

			var got EntrySlice
			if gotStream := s.Filter(tt.args.predicate).stream; gotStream != nil {
				got = EntrySlice{}
				for val := range gotStream {
					got = append(got, val)
				}
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Stream.Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStream_ForEach(t *testing.T) {
	var callCount, total int
	computeSumTotal := func(value Entry) {
		callCount++
		total += int(value.(EntryInt))
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		consumer Consumer
	}
	type want struct {
		total, count int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   want
	}{
		{
			name:   "Should not call consumer for a Stream of nil",
			fields: fields{stream: nil},
			args: args{
				consumer: computeSumTotal,
			},
			want: want{
				count: 0,
				total: 0,
			},
		},
		{
			name: "Should give produce filtered values as per predicate",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() {
						defer close(c)
						c <- EntryInt(4)
						c <- EntryInt(1)
						c <- EntryInt(3)
					}()
					return c
				}()},
			args: args{
				consumer: computeSumTotal,
			},
			want: want{
				count: 3,
				total: 8,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount, total = 0, 0
			s := Stream{
				stream: tt.fields.stream,
			}
			s.ForEach(tt.args.consumer)
			assert.Equal(t, tt.want.count, callCount)
			assert.Equal(t, tt.want.total, total)
		})
	}
}

func concatenateStringsBiFunc(i, j Entry) Entry {
	iStr := i.(EntryString)
	jStr := j.(EntryString)
	return iStr + "-" + jStr
}

func TestStream_LeftReduce(t *testing.T) {
	type fields struct {
		stream chan Entry
	}
	type args struct {
		f2 BiFunction
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   Entry
	}{
		{
			name:   "Should return nil for a Stream of nil",
			fields: fields{stream: nil},
			args:   args{f2: concatenateStringsBiFunc},
			want:   nil,
		},
		{
			name: "Should return reduction of set of single element",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() {
						defer close(c)
						c <- EntryString("three")
					}()
					return c
				}()},
			args: args{f2: concatenateStringsBiFunc},
			want: EntryString("three"),
		},
		{
			name: "Should return reduction of set of multiple elements",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() {
						defer close(c)
						c <- EntryString("four")
						c <- EntryString("twelve")
						c <- EntryString("one")
						c <- EntryString("six")
						c <- EntryString("three")
					}()
					return c
				}()},
			args: args{f2: concatenateStringsBiFunc},
			want: EntryString("four-twelve-one-six-three"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			if gotReduce := s.Reduce(tt.args.f2); !assert.Exactly(t, tt.want, gotReduce) {
				return
			}
		})
	}
}

func TestStream_Intersperse(t *testing.T) {
	type fields struct {
		stream chan Entry
	}
	type args struct {
		e Entry
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntrySlice
	}{
		{
			name:   "Should return an empty Stream for nil input Stream",
			fields: fields{stream: nil},
			args: args{
				e: EntryString(" - "),
			},
			want: EntrySlice{},
		},
		{
			name: "Should return an empty Stream for empty input Stream",
			fields: fields{stream: func() chan Entry {
				c := make(chan Entry)
				go func() {
					defer close(c)
				}()
				return c
			}()},
			args: args{
				e: EntryString(" - "),
			},
			want: EntrySlice{},
		},
		{
			name: "Should return the original input Stream when it has a single value",
			fields: fields{stream: func() chan Entry {
				c := make(chan Entry)
				go func() {
					defer close(c)
					c <- EntryString("four")
				}()
				return c
			}()},
			args: args{
				e: EntryString(" - "),
			},
			want: EntrySlice{
				EntryString("four"),
			},
		},
		{
			name: "Should return the Set with given value interspersed",
			fields: fields{stream: func() chan Entry {
				c := make(chan Entry)
				go func() {
					defer close(c)
					c <- EntryString("four")
					c <- EntryString("twelve")
					c <- EntryString("one")
					c <- EntryString("six")
					c <- EntryString("three")
				}()
				return c
			}()},
			args: args{
				e: EntryString(" - "),
			},
			want: EntrySlice{
				EntryString("four"),
				EntryString(" - "),
				EntryString("twelve"),
				EntryString(" - "),
				EntryString("one"),
				EntryString(" - "),
				EntryString("six"),
				EntryString(" - "),
				EntryString("three")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			out := s.Intersperse(tt.args.e)
			got := EntrySlice{}
			for e := range out.stream {
				got = append(got, e)
			}
			if !assert.ElementsMatch(t, got, tt.want) {
				t.Errorf("Stream.Intersperse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStream_GroupBy(t *testing.T) {
	type fields struct {
		stream chan Entry
	}
	type args struct {
		classifier Function
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntryMap
	}{
		{
			name: "Should return empty map when iterator with nil stream",
			fields: fields{
				stream: nil,
			},
			args: args{
				classifier: func(i Entry) Entry {
					return i.(EntryInt) & 1
				},
			},
			want: EntryMap{},
		},
		{
			name: "Should return empty map when empty stream",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() {
						defer close(c)
					}()
					return c
				}(),
			},
			args: args{
				classifier: func(i Entry) Entry {
					return i.(EntryInt) & 1
				},
			},
			want: EntryMap{},
		},
		{
			name: "Should group by odd / even numbers",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() {
						defer close(c)
						c <- EntryInt(1)
						c <- EntryInt(2)
						c <- EntryInt(3)
						c <- EntryInt(4)
					}()
					return c
				}(),
			},
			args: args{
				classifier: func(i Entry) Entry {
					return i.(EntryInt) & 1
				},
			},
			want: EntryMap{
				EntryInt(0): EntrySlice{
					EntryInt(2),
					EntryInt(4),
				},
				EntryInt(1): EntrySlice{
					EntryInt(1),
					EntryInt(3),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp := Stream{
				stream: tt.fields.stream,
			}
			got := rp.GroupBy(tt.args.classifier)
			assert.True(t, got.Equal(tt.want))
		})
	}
}

func TestStream_NewStreamFromNilChannelPanics(t *testing.T) {
	assert.PanicsWithValue(t, PanicMissingChannel, func() { NewStream(nil) })
}

func TestStream_NewStream(t *testing.T) {
	emptyChannel := make(chan Entry)
	populatedChannel := func() chan Entry {
		c := make(chan Entry)
		go func() {
			defer close(c)
			c <- EntryInt(1)
			c <- EntryInt(2)
			c <- EntryInt(3)
			c <- EntryInt(4)
		}()
		return c
	}()

	type args struct {
		s chan Entry
	}
	tests := []struct {
		name string
		args args
		want Stream
	}{
		{
			name: "Should create an empty Stream with an empty channel",
			args: args{s: emptyChannel},
			want: Stream{stream: emptyChannel},
		},
		{
			name: "Should create a Stream with a populated channel",
			args: args{
				s: populatedChannel,
			},
			want: Stream{
				stream: populatedChannel,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewStream(tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewStream() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStream_NewStreamFromSlice(t *testing.T) {
	type args struct {
		s EntrySlice
	}
	tests := []struct {
		name string
		args args
		want EntrySlice
	}{
		{
			name: "Should create a Stream with an empty channel",
			args: args{s: nil},
			want: EntrySlice{},
		},
		{
			name: "Should create an empty Stream with an empty channel",
			args: args{s: EntrySlice{}},
			want: EntrySlice{},
		},
		{
			name: "Should create a Stream with a populated channel",
			args: args{
				s: EntrySlice{
					EntryInt(1),
					EntryInt(2),
					EntryInt(3),
					EntryInt(4),
				},
			},
			want: EntrySlice{
				EntryInt(1),
				EntryInt(2),
				EntryInt(3),
				EntryInt(4),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got EntrySlice
			if gotStream := NewStreamFromSlice(tt.args.s, 0).stream; gotStream != nil {
				got = EntrySlice{}
				for val := range gotStream {
					got = append(got, val)
				}
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewStreamFromSlice() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestStream_Count(t *testing.T) {
	type fields struct {
		stream chan Entry
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name:   "Should return 0 for a nil channel",
			fields: fields{stream: nil},
			want:   0,
		},
		{
			name: "Should return 0 for an empty closed channel",
			fields: fields{stream: func() chan Entry {
				c := make(chan Entry)
				go func() {
					defer close(c)
				}()
				return c
			}()},
			want: 0,
		},
		{
			name: "Should return 3 for a size 3 closed channel",
			fields: fields{stream: func() chan Entry {
				c := make(chan Entry, 1)
				go func() {
					defer close(c)
					c <- EntryInt(1)
					c <- EntryInt(2)
					c <- EntryInt(1)
				}()
				return c
			}()},
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			if got := s.Count(); got != tt.want {
				t.Errorf("Stream.Count() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStream_MapToInt(t *testing.T) {
	entryToInt := func(e Entry) EntryInt {
		switch j := e.(type) {
		case EntryInt:
			return j
		case EntryString:
			return EntryInt(j[0])
		default:
			return EntryInt(0xdeadbeef)
		}
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		toInt ToIntFunction
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   IntStream
	}{
		{
			name:   "Should map a nil stream to an empty stream of EntryInt's",
			fields: fields{stream: nil},
			args: args{
				toInt: entryToInt,
			},
			want: NewIntStreamFromSlice([]EntryInt{}, 0),
		},
		{
			name: "Should map a stream of Entry's to a stream of EntryInt's",
			fields: fields{stream: func() chan Entry {
				c := make(chan Entry, 2)
				go func() {
					defer close(c)
					c <- EntryString("a")
					c <- EntryBool(false)
					c <- EntryString("b")
					c <- EntryInt(-17)
					c <- EntryString("c")
				}()
				return c
			}()},
			args: args{
				toInt: func(e Entry) EntryInt {
					switch j := e.(type) {
					case EntryInt:
						return j
					case EntryString:
						return EntryInt(j[0])
					default:
						return EntryInt(0xdeadbeef)
					}
				},
			},
			want: NewIntStreamFromSlice([]EntryInt{
				EntryInt(97),
				EntryInt(0xdeadbeef),
				EntryInt(98),
				EntryInt(-17),
				EntryInt(99),
			}, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			gotIntStream := s.MapToInt(tt.args.toInt)

			if !assert.IsType(t, IntStream{}, gotIntStream) {
				t.Errorf("Stream.MapToInt() did not return type = %T, want IntStream", gotIntStream)
			}

			wantStr := ""
			for v := range tt.want.stream {
				wantStr += fmt.Sprintf("%v ", v)
			}
			gotStr := ""
			for v := range gotIntStream.stream {
				gotStr += fmt.Sprintf("%v ", v)
			}
			if !assert.Equal(t, wantStr, gotStr) {
				t.Errorf("Stream.MapToInt() = %v, want %v", gotStr, wantStr)
			}
		})
	}
}

func TestStream_MapToFloat(t *testing.T) {
	entryToFloat := func(e Entry) EntryFloat {
		switch j := e.(type) {
		case EntryFloat:
			return j
		case EntryString:
			return EntryFloat(j[0])
		default:
			return EntryFloat(0xdeadbeef)
		}
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		toFloat ToFloatFunction
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   FloatStream
	}{
		{
			name:   "Should map a nil stream to an empty stream of EntryFloat's",
			fields: fields{stream: nil},
			args: args{
				toFloat: entryToFloat,
			},
			want: NewFloatStreamFromSlice([]EntryFloat{}, 0),
		},
		{
			name: "Should map a stream of Entry's to a stream of EntryFloat's",
			fields: fields{stream: func() chan Entry {
				c := make(chan Entry, 2)
				go func() {
					defer close(c)
					c <- EntryString("a")
					c <- EntryBool(false)
					c <- EntryString("b")
					c <- EntryFloat(-17)
					c <- EntryString("c")
				}()
				return c
			}()},
			args: args{
				toFloat: func(e Entry) EntryFloat {
					switch j := e.(type) {
					case EntryFloat:
						return j
					case EntryString:
						return EntryFloat(j[0])
					default:
						return EntryFloat(0xdeadbeef)
					}
				},
			},
			want: NewFloatStreamFromSlice([]EntryFloat{
				EntryFloat(97),
				EntryFloat(0xdeadbeef),
				EntryFloat(98),
				EntryFloat(-17),
				EntryFloat(99),
			}, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			gotFloatStream := s.MapToFloat(tt.args.toFloat)

			if !assert.IsType(t, FloatStream{}, gotFloatStream) {
				t.Errorf("Stream.MapToFloat() did not return type = %T, want FloatStream", gotFloatStream)
			}

			wantStr := ""
			for v := range tt.want.stream {
				wantStr += fmt.Sprintf("%v ", v)
			}
			gotStr := ""
			for v := range gotFloatStream.stream {
				gotStr += fmt.Sprintf("%v ", v)
			}
			if !assert.Equal(t, wantStr, gotStr) {
				t.Errorf("Stream.MapToFloat() = %v, want %v", gotStr, wantStr)
			}
		})
	}
}

func TestStream_AnyMatch(t *testing.T) {
	dataGenerator := func() chan Entry {
		c := make(chan Entry, 2)
		go func() {
			defer close(c)
			c <- EntryString("a")
			c <- EntryBool(false)
			c <- EntryString("b")
			c <- EntryInt(-17)
			c <- EntryString("c")
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		p Predicate
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "Should not match any when channel is nil",
			fields: fields{stream: nil},
			args:   args{p: True},
			want:   false,
		},
		{
			name:   "Should not match any",
			fields: fields{stream: dataGenerator()},
			args:   args{p: func(e Entry) bool { return e.Equal(EntryString("not in here")) }},
			want:   false,
		},
		{
			name:   "Should match any",
			fields: fields{stream: dataGenerator()},
			args:   args{p: func(e Entry) bool { return e.Equal(EntryString("b")) }},
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			if got := s.AnyMatch(tt.args.p); got != tt.want {
				t.Errorf("Stream.AnyMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStream_NoneMatch(t *testing.T) {
	dataGenerator := func() chan Entry {
		c := make(chan Entry, 2)
		go func() {
			defer close(c)
			c <- EntryString("a")
			c <- EntryBool(false)
			c <- EntryString("b")
			c <- EntryInt(-17)
			c <- EntryString("c")
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		p Predicate
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "Should satisfy when channel is nil",
			fields: fields{stream: nil},
			args:   args{p: True},
			want:   true,
		},
		{
			name:   "Should satisfy",
			fields: fields{stream: dataGenerator()},
			args:   args{p: func(e Entry) bool { return e.Equal(EntryString("not in here")) }},
			want:   true,
		},
		{
			name:   "Should not satisfy",
			fields: fields{stream: dataGenerator()},
			args:   args{p: func(e Entry) bool { return e.Equal(EntryString("b")) }},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			if got := s.NoneMatch(tt.args.p); got != tt.want {
				t.Errorf("Stream.NoneMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStream_AllMatch(t *testing.T) {
	dataGenerator := func() chan Entry {
		c := make(chan Entry, 2)
		go func() {
			defer close(c)
			c <- EntryString("a")
			c <- EntryBool(false)
			c <- EntryString("b")
			c <- EntryInt(-17)
			c <- EntryString("c")
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		p Predicate
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "Should not match all when channel is nil",
			fields: fields{stream: nil},
			args:   args{p: True},
			want:   false,
		},
		{
			name:   "Should match all",
			fields: fields{stream: dataGenerator()},
			args:   args{p: func(e Entry) bool { return !e.Equal(EntryString("not in here")) }},
			want:   true,
		},
		{
			name:   "Should not match all",
			fields: fields{stream: dataGenerator()},
			args:   args{p: func(e Entry) bool { return e.Equal(EntryString("b")) }},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			if got := s.AllMatch(tt.args.p); got != tt.want {
				t.Errorf("Stream.AllMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStream_Drop(t *testing.T) {
	data1 := EntrySlice{
		EntryInt(1),
	}

	data := EntrySlice{
		EntryString("a"),
		EntryBool(false),
		EntryString("b"),
		EntryInt(-17),
		EntryString("c"),
	}

	dataGenerator := func(slice EntrySlice) chan Entry {
		c := make(chan Entry, 2)
		go func() {
			defer close(c)
			for _, val := range slice {
				c <- val
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		n uint64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntrySlice
	}{
		{
			name: "Should return an empty stream when nil channel",
			fields: fields{
				stream: nil,
			},
			args: args{
				n: 1,
			},
			want: EntrySlice{},
		},
		{
			name: "Should not change the stream if n < 1",
			fields: fields{
				stream: dataGenerator(data),
			},
			args: args{
				n: 0,
			},
			want: data,
		},
		{
			name: "Should drop all elements when n > number of elements in the stream",
			fields: fields{
				stream: dataGenerator(data),
			},
			args: args{
				n: uint64(len(data) + 10),
			},
			want: EntrySlice{},
		},
		{
			name: "Should drop the first n elements when n < number of elements in the stream",
			fields: fields{
				stream: dataGenerator(data),
			},
			args: args{
				n: 2,
			},
			want: data[2:],
		},
		{
			name: "Should drop the sole element in the stream",
			fields: fields{
				stream: dataGenerator(data1),
			},
			args: args{
				n: 1,
			},
			want: EntrySlice{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{stream: tt.fields.stream}
			gotStream := s.Drop(tt.args.n)
			if tt.want == nil {
				assert.Nil(t, gotStream.stream)
				return
			}
			got := EntrySlice{}
			for val := range gotStream.stream {
				got = append(got, val)
			}
			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestStream_DropWhile(t *testing.T) {
	data := EntrySlice{
		EntryString("a"),
		EntryBool(false),
		EntryString("b"),
		EntryInt(-17),
		EntryString("c"),
	}
	dataGenerator := func() chan Entry {
		c := make(chan Entry, 2)
		go func() {
			defer close(c)
			for _, val := range data {
				c <- val
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		p Predicate
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntrySlice
	}{
		{
			name: "Should return empty out-stream when nil in-stream",
			fields: fields{
				stream: nil,
			},
			args: args{
				p: True,
			},
			want: EntrySlice{},
		},
		{
			name: "Should not change the stream if predicate never satisfies",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: False,
			},
			want: data,
		},
		{
			name: "Should drop the first few elements that satisfy the predicate",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: func(e Entry) bool {
					return e.Equal(EntryString("a")) || e.Equal(EntryBool(false))
				},
			},
			want: data[2:],
		},
		{
			name: "Should drop all elements when the predicate always satisfies",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: True,
			},
			want: EntrySlice{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{stream: tt.fields.stream}
			gotStream := s.DropWhile(tt.args.p)
			got := EntrySlice{}
			for val := range gotStream.stream {
				got = append(got, val)
			}
			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestStream_DropUntil(t *testing.T) {
	data := EntrySlice{
		EntryString("a"),
		EntryBool(false),
		EntryString("b"),
		EntryInt(-17),
		EntryString("c"),
	}
	dataGenerator := func() chan Entry {
		c := make(chan Entry, 2)
		go func() {
			defer close(c)
			for _, val := range data {
				c <- val
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		p Predicate
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntrySlice
	}{
		{
			name: "Should return empty out-stream when nil in-stream",
			fields: fields{
				stream: nil,
			},
			args: args{
				p: True,
			},
			want: EntrySlice{},
		},
		{
			name: "Should return empty stream if predicate never satisfies",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: False,
			},
			want: EntrySlice{},
		},
		{
			name: "Should drop the first few elements that satisfy the predicate",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: func(e Entry) bool {
					return e.Equal(EntryString("b"))
				},
			},
			want: EntrySlice{
				EntryString("b"),
				EntryInt(-17),
				EntryString("c"),
			},
		},
		{
			name: "Should not drop any element when the predicate always satisfies",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: True,
			},
			want: data,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{stream: tt.fields.stream}
			gotStream := s.DropUntil(tt.args.p)
			got := EntrySlice{}
			for val := range gotStream.stream {
				got = append(got, val)
			}
			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestStream_LastX_PanicsWhenNilChannel(t *testing.T) {
	assert.PanicsWithValue(t, PanicMissingChannel, func() { Stream{stream: nil}.LastN(1) })
	assert.PanicsWithValue(t, PanicMissingChannel, func() { Stream{stream: nil}.Last() })
}

func TestStream_LastX_PanicsWhenEmptyChannel(t *testing.T) {
	emptyStream := func() chan Entry {
		c := make(chan Entry)
		go func() {
			defer close(c)
		}()
		return c
	}

	assert.PanicsWithValue(t, PanicNoSuchElement, func() { NewStream(emptyStream()).LastN(1) })
	assert.PanicsWithValue(t, PanicNoSuchElement, func() { NewStream(emptyStream()).Last() })
}

func TestStream_LastNWithInvalidArgumentPanics(t *testing.T) {
	tests := []struct {
		name      string
		n         uint64
		wantPanic bool
	}{
		{
			name:      "Should panic when N is less than 1",
			n:         0,
			wantPanic: true,
		},
		{
			name:      "Should not panic when N is 1",
			n:         1,
			wantPanic: false,
		},
	}

	populatedStream := func() chan Entry {
		c := make(chan Entry)
		go func() {
			defer close(c)
			c <- EntryString("joy")
		}()
		return c
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: populatedStream(),
			}
			if tt.wantPanic {
				assert.PanicsWithValue(t, PanicNoSuchElement, func() { s.LastN(tt.n) })
			} else {
				assert.NotPanics(t, func() { s.LastN(tt.n) })
			}
		})
	}
}

func TestStream_LastN(t *testing.T) {
	data := EntrySlice{
		EntryString("one"),
		EntryString("two"),
		EntryString("thee"),
		EntryString("four"),
		EntryString("five"),
	}

	largeData := EntrySlice{}
	for i := 1; i < 1e5; i++ {
		largeData = append(largeData, EntryInt(i))
	}

	populatedStream := func(slice EntrySlice) chan Entry {
		c := make(chan Entry)
		go func() {
			defer close(c)
			for _, val := range slice {
				c <- val
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		n uint64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntrySlice
	}{
		{
			name: "Should return the last n elements when the stream is longer",
			fields: fields{
				stream: populatedStream(data),
			},
			args: args{2},
			want: data[3:],
		},
		{
			name: "Should return all the elements when the stream is shorter",
			fields: fields{
				stream: populatedStream(data),
			},
			args: args{uint64(len(data) + 10)},
			want: data,
		},
		{
			name: "Should return the last n elements when the stream is significantly larger",
			fields: fields{
				stream: populatedStream(largeData),
			},
			args: args{2e3},
			want: largeData[len(largeData)-2e3:],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			got := s.LastN(tt.args.n)
			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestStream_EndsWith(t *testing.T) {
	data0 := EntrySlice{}
	data1 := EntrySlice{EntryInt(16)}
	data4 := EntrySlice{
		EntryBool(true),
		EntryInt(1),
		EntryInt(4),
		EntryString("two"),
	}
	data5 := EntrySlice{
		EntryInt(1),
		EntryString("two"),
		EntryBool(true),
		EntryInt(4),
		EntryString("five"),
	}

	generateStream := func(data EntrySlice) chan Entry {
		c := make(chan Entry)
		go func() {
			defer close(c)
			for _, e := range data {
				c <- e
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		slice EntrySlice
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "Should not match with a nil channel",
			fields: fields{stream: nil},
			args:   args{slice: data1},
			want:   false,
		},
		{
			name:   "Should not match with an empty stream and with input",
			fields: fields{stream: generateStream(data0)},
			args:   args{slice: data1},
			want:   false,
		},
		{
			name:   "Should not match with an empty stream and with no input",
			fields: fields{stream: generateStream(data0)},
			args:   args{slice: data0},
			want:   false,
		},
		{
			name:   "Should not match when stream size is less than slice to compare even when the elements match",
			fields: fields{stream: generateStream(data5[:3])},
			args:   args{slice: data5},
			want:   false,
		},
		{
			name:   "Should match when stream size and data match slice to compare",
			fields: fields{stream: generateStream(data5)},
			args:   args{slice: data5},
			want:   true,
		},
		{
			name:   "Should match when the stream ends with the slice to compare",
			fields: fields{stream: generateStream(data5)},
			args:   args{slice: data5[3:]},
			want:   true,
		},
		{
			name:   "Should not match when stream does not end with slice to compare",
			fields: fields{stream: generateStream(data5)},
			args:   args{slice: data4},
			want:   false,
		},
		{
			name:   "Should not match when stream is shorter than slice to compare",
			fields: fields{stream: generateStream(data5[:2])},
			args:   args{slice: data5},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			assert.Equal(t, tt.want, s.EndsWith(tt.args.slice))
		})
	}
}

func TestStream_HeadX_PanicsWhenNilChannel(t *testing.T) {
	assert.PanicsWithValue(t, PanicMissingChannel, func() { Stream{stream: nil}.HeadN(1) })
	assert.PanicsWithValue(t, PanicMissingChannel, func() { Stream{stream: nil}.Head() })
}

func TestStream_Head_PanicsWhenEmptyChannel(t *testing.T) {
	emptyStream := func() chan Entry {
		c := make(chan Entry)
		go func() {
			defer close(c)
		}()
		return c
	}

	assert.PanicsWithValue(t, PanicNoSuchElement, func() { NewStream(emptyStream()).Head() })
}

func TestStream_Head(t *testing.T) {
	data1 := EntrySlice{
		EntryString("one"),
	}
	data5 := EntrySlice{
		EntryString("one"),
		EntryString("two"),
		EntryString("thee"),
		EntryString("four"),
		EntryString("five"),
	}

	generateStream := func(slice EntrySlice) chan Entry {
		c := make(chan Entry)
		go func() {
			defer close(c)
			for _, val := range slice {
				c <- val
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	tests := []struct {
		name   string
		fields fields
		want   Entry
	}{
		{
			name: "Should return first element when the stream has one element",
			fields: fields{
				stream: generateStream(data1),
			},
			want: EntryString("one"),
		},
		{
			name: "Should return first element when the stream has multiple elements",
			fields: fields{
				stream: generateStream(data5),
			},
			want: EntryString("one"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			if got := s.Head(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Stream.Head() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStream_HeadN(t *testing.T) {
	data0 := EntrySlice{}

	data := EntrySlice{
		EntryString("one"),
		EntryString("two"),
		EntryString("thee"),
		EntryString("four"),
		EntryString("five"),
	}

	largeData := EntrySlice{}
	for i := 1; i < 1e5; i++ {
		largeData = append(largeData, EntryInt(i))
	}

	generateStream := func(slice EntrySlice) chan Entry {
		c := make(chan Entry)
		go func() {
			defer close(c)
			for _, val := range slice {
				c <- val
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		n uint64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntrySlice
	}{
		{
			name: "Should return empty slice when the stream is empty",
			fields: fields{
				stream: generateStream(data0),
			},
			args: args{2},
			want: EntrySlice{},
		},
		{
			name: "Should return the first n elements when the stream is longer",
			fields: fields{
				stream: generateStream(data),
			},
			args: args{2},
			want: data[:2],
		},
		{
			name: "Should return all the elements when the stream is shorter",
			fields: fields{
				stream: generateStream(data),
			},
			args: args{2e3},
			want: data,
		},
		{
			name: "Should return the first n elements when the stream is significantly larger",
			fields: fields{
				stream: generateStream(largeData),
			},
			args: args{2e3},
			want: largeData[:2000],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			got := s.HeadN(tt.args.n)
			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestStream_StartsWithPanicsWithNilChannel(t *testing.T) {
	s := Stream{stream: nil}
	assert.PanicsWithValue(t, PanicMissingChannel, func() { s.StartsWith(EntrySlice{}) })
}

func TestStream_StartsWith(t *testing.T) {
	data0 := EntrySlice{}
	data1 := EntrySlice{EntryInt(16)}
	data4 := EntrySlice{
		EntryBool(true),
		EntryInt(1),
		EntryInt(4),
		EntryString("two"),
	}
	data5 := EntrySlice{
		EntryInt(1),
		EntryString("two"),
		EntryBool(true),
		EntryInt(4),
		EntryString("five"),
	}

	generateStream := func(data EntrySlice) chan Entry {
		c := make(chan Entry)
		go func() {
			defer close(c)
			for _, e := range data {
				c <- e
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		slice EntrySlice
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "Should not match with an empty stream",
			fields: fields{stream: generateStream(data0)},
			args:   args{slice: data1},
			want:   false,
		},
		{
			name:   "Should not match with an empty slice: case with an empty stream ",
			fields: fields{stream: generateStream(data0)},
			args:   args{slice: data0},
			want:   false,
		},
		{
			name:   "Should not match with an empty slice: case with a populated stream ",
			fields: fields{stream: generateStream(data1)},
			args:   args{slice: data0},
			want:   false,
		},
		{
			name:   "Should not match when stream size is less than slice to compare even when the elements match",
			fields: fields{stream: generateStream(data5[:3])},
			args:   args{slice: data5},
			want:   false,
		},
		{
			name:   "Should match when stream size and data matches slice to compare",
			fields: fields{stream: generateStream(data5)},
			args:   args{slice: data5},
			want:   true,
		},
		{
			name:   "Should match when stream starts with the slice to compare",
			fields: fields{stream: generateStream(data5)},
			args:   args{slice: data5[:3]},
			want:   true,
		},
		{
			name:   "Should not match when stream does not end with slice to compare",
			fields: fields{stream: generateStream(data5)},
			args:   args{slice: data4},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			assert.Equal(t, tt.want, s.StartsWith(tt.args.slice))
		})
	}
}

func TestStream_TakeX_LimitPanicWithNilChannel(t *testing.T) {
	s := Stream{stream: nil}
	assert.PanicsWithValue(t, PanicMissingChannel, func() { s.Limit(1) })
	assert.PanicsWithValue(t, PanicMissingChannel, func() { s.TakeUntil(False) })
	assert.PanicsWithValue(t, PanicMissingChannel, func() { s.TakeWhile(True) })
}

func TestStream_Take_Limit(t *testing.T) {
	data1 := EntrySlice{
		EntryInt(1),
	}

	data := EntrySlice{
		EntryString("a"),
		EntryBool(false),
		EntryString("b"),
		EntryInt(-17),
		EntryString("c"),
	}

	dataGenerator := func(slice EntrySlice) chan Entry {
		c := make(chan Entry, 2)
		go func() {
			defer close(c)
			for _, val := range slice {
				c <- val
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		n uint64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntrySlice
	}{
		{
			name: "Should return empty stream when n < 1",
			fields: fields{
				stream: dataGenerator(data),
			},
			args: args{
				n: 0,
			},
			want: EntrySlice{},
		},
		{
			name: "Should return all elements when n > number of elements in the stream",
			fields: fields{
				stream: dataGenerator(data),
			},
			args: args{
				n: uint64(len(data) + 10),
			},
			want: data,
		},
		{
			name: "Should return the first n elements when n < number of elements in the stream",
			fields: fields{
				stream: dataGenerator(data),
			},
			args: args{
				n: 2,
			},
			want: data[:2],
		},
		{
			name: "Should return the sole element in the stream",
			fields: fields{
				stream: dataGenerator(data1),
			},
			args: args{
				n: 1,
			},
			want: data1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{stream: tt.fields.stream}
			gotStream := s.Limit(tt.args.n)
			if tt.want == nil {
				assert.Nil(t, gotStream.stream)
				return
			}
			got := EntrySlice{}
			for val := range gotStream.stream {
				got = append(got, val)
			}
			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestStream_TakeWhile(t *testing.T) {
	data := EntrySlice{
		EntryString("a"),
		EntryBool(false),
		EntryString("b"),
		EntryInt(-17),
		EntryString("c"),
	}
	dataGenerator := func() chan Entry {
		c := make(chan Entry, 2)
		go func() {
			defer close(c)
			for _, val := range data {
				c <- val
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		p Predicate
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntrySlice
	}{
		{
			name: "Should return empty stream if predicate never satisfies",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: False,
			},
			want: EntrySlice{},
		},
		{
			name: "Should take the first few elements that satisfy the predicate",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: func(e Entry) bool {
					return e.Equal(EntryString("a")) || e.Equal(EntryBool(false))
				},
			},
			want: data[:2],
		},
		{
			name: "Should take all elements when the predicate always satisfies",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: True,
			},
			want: data,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{stream: tt.fields.stream}
			gotStream := s.TakeWhile(tt.args.p)
			got := EntrySlice{}
			for val := range gotStream.stream {
				got = append(got, val)
			}
			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestStream_TakeUntil(t *testing.T) {
	data := EntrySlice{
		EntryString("a"),
		EntryBool(false),
		EntryString("b"),
		EntryInt(-17),
		EntryString("c"),
	}
	dataGenerator := func() chan Entry {
		c := make(chan Entry, 2)
		go func() {
			defer close(c)
			for _, val := range data {
				c <- val
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		p Predicate
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   EntrySlice
	}{
		{
			name: "Should return whole stream if predicate never satisfies",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: False,
			},
			want: data,
		},
		{
			name: "Should take the first few elements until predicate satisfies",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: func(e Entry) bool {
					return e.Equal(EntryString("b"))
				},
			},
			want: EntrySlice{
				EntryString("a"),
				EntryBool(false),
			},
		},
		{
			name: "Should return empty stream when the predicate always satisfies",
			fields: fields{
				stream: dataGenerator(),
			},
			args: args{
				p: True,
			},
			want: EntrySlice{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{stream: tt.fields.stream}
			gotStream := s.TakeUntil(tt.args.p)
			got := EntrySlice{}
			for val := range gotStream.stream {
				got = append(got, val)
			}
			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestStream_CollectPanicsWhenStreamIsNil(t *testing.T) {
	supplier := func() Entry {
		return EntrySlice{}
	}
	accumulator := func(e1, e2 Entry) Entry {
		return e1.(EntrySlice).Append(e2)
	}

	s := Stream{stream: nil}
	assert.PanicsWithValue(t, PanicMissingChannel, func() { s.Collect(NewCollector(supplier, accumulator, nil)) })
}
func TestStream_Collect(t *testing.T) {
	supplier := func() Entry {
		return EntrySlice{}
	}
	accumulator := func(e1, e2 Entry) Entry {
		return e1.(EntrySlice).Append(e2)
	}
	finisher := func(e Entry) Entry {
		total := EntryInt(0)
		for _, e1 := range e.(EntrySlice) {
			total += e1.(EntryInt)
		}
		return total
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		c Collector
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   interface{}
	}{
		{
			name: "Should not call finisher when nil (it would crash otherwise)",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() {
						defer close(c)
						c <- EntryInt(2)
						c <- EntryInt(3)
						c <- EntryInt(4)
						c <- EntryInt(5)
					}()
					return c
				}(),
			},
			args: args{
				c: NewCollector(supplier, accumulator, nil),
			},
			want: EntrySlice{
				EntryInt(2),
				EntryInt(3),
				EntryInt(4),
				EntryInt(5),
			},
		},
		{
			name: "Should call finisher when provided",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() {
						defer close(c)
						c <- EntryInt(2)
						c <- EntryInt(3)
						c <- EntryInt(4)
						c <- EntryInt(5)
					}()
					return c
				}(),
			},
			args: args{
				c: NewCollector(supplier, accumulator, finisher),
			},
			want: EntryInt(14),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			if got := s.Collect(tt.args.c); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Stream.Collect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStream_FlatMap(t *testing.T) {
	type fields struct {
		stream chan Entry
	}
	type args struct {
		mapper StreamFunction
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   Stream
	}{
		{
			name: "Should produce an empty stream when in-stream is nil",
			fields: fields{
				stream: nil,
			},
			args: args{
				mapper: func(e Entry) Stream {
					return FlattenEntrySliceToEntry(0)(e)
				},
			},
			want: NewStreamFromSlice(EntrySlice{}, 0),
		},
		{
			name: "Should produce an empty stream when in-stream is empty",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() { defer close(c) }()
					return c
				}(),
			},
			args: args{
				mapper: func(e Entry) Stream {
					return FlattenEntrySliceToEntry(0)(e)
				},
			},
			want: NewStreamFromSlice(EntrySlice{}, 0),
		},
		{
			name: "Should produce a flat stream when in-stream is not empty",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() {
						defer close(c)
						c <- EntrySlice{
							EntryInt(1),
							EntryInt(2),
							EntryInt(3),
						}
						c <- EntrySlice{
							EntryInt(4),
							EntryInt(5),
							EntryInt(6),
						}
					}()
					return c
				}(),
			},
			args: args{
				mapper: func(e Entry) Stream {
					return FlattenEntrySliceToEntry(0)(e)
				},
			},
			want: NewStreamFromSlice(EntrySlice{
				EntryInt(1),
				EntryInt(2),
				EntryInt(3),
				EntryInt(4),
				EntryInt(5),
				EntryInt(6),
			}, 0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			got := s.FlatMap(tt.args.mapper).ToSlice()
			expected := tt.want.ToSlice()
			assert.EqualValues(t, expected, got)
		})
	}
}

func TestStream_FlatMap_Concurrent(t *testing.T) {
	const numSlices = 300
	const concurrencyLevel = numSlices / 10

	sourceStream := func() chan Entry {
		c := make(chan Entry, 10)
		go func() {
			defer close(c)
			for i := 0; i < numSlices; i++ {
				es := EntrySlice{}
				for j := 0; j < 10; j++ {
					es = append(es, EntryInt(j))
				}
				c <- es
			}
		}()
		return c
	}()

	// Stream.flatMap() helps in converting Collection<Collection<T>> to Collection<T>.
	// flatMap() = map() + Flattening
	mapper := func(e Entry) Stream {
		time.Sleep(50 * time.Millisecond)
		return FlattenEntrySliceToEntry(concurrencyLevel)(e)
	}

	want := func() EntrySlice {
		resultES := EntrySlice{}
		for i := 0; i < numSlices; i++ {
			for j := 0; j < 10; j++ {
				resultES = resultES.Append(EntryInt(j))
			}
		}
		return resultES
	}()

	unitStream := Stream{
		stream: sourceStream,
	}
	gotStream := unitStream.Concurrent(concurrencyLevel).FlatMap(mapper).ToSlice()

	if !reflect.DeepEqual(gotStream, want) {
		t.Errorf("Stream.Map() = %v, want %v", gotStream, want)
	}
}

func TestStream_ToSlice(t *testing.T) {
	type fields struct {
		stream chan Entry
	}
	tests := []struct {
		name   string
		fields fields
		want   EntrySlice
	}{
		{
			name: "Should produce an empty stream when in-stream is nil",
			fields: fields{
				stream: nil,
			},
			want: EntrySlice{},
		},
		{
			name: "Should produce an empty stream when in-stream is empty",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() { defer close(c) }()
					return c
				}(),
			},
			want: EntrySlice{},
		},
		{
			name: "Should produce a flat stream when in-stream is not empty",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() {
						defer close(c)
						c <- EntryInt(1)
						c <- EntryInt(2)
						c <- EntryInt(3)
					}()
					return c
				}(),
			},
			want: EntrySlice{
				EntryInt(1),
				EntryInt(2),
				EntryInt(3),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			got := s.ToSlice()
			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestStream_DistinctPanicsWhenNilChannel(t *testing.T) {
	assert.PanicsWithValue(t, PanicMissingChannel, func() { Stream{stream: nil}.Distinct() })
}

func TestStream_Distinct(t *testing.T) {
	data5 := EntrySlice{
		EntryInt(1),
		EntryString("two"),
		EntryBool(true),
		EntryInt(4),
		EntryString("five"),
	}

	data10 := EntrySlice{
		EntryInt(1),
		EntryString("five"),
		EntryString("two"),
		EntryInt(1),
		EntryString("five"),
		EntryString("five"),
		EntryBool(true),
		EntryInt(4),
		EntryBool(false),
		EntryString("five"),
	}

	generateStream := func(data EntrySlice) chan Entry {
		c := make(chan Entry, 200)
		go func() {
			defer close(c)
			for _, e := range data {
				c <- e
			}
		}()
		return c
	}

	type fields struct {
		stream chan Entry
	}
	tests := []struct {
		name   string
		fields fields
		want   Stream
	}{
		{
			name: "Should return same values when the in-stream has distinct values",
			fields: fields{
				stream: generateStream(data5),
			},
			want: NewStreamFromSlice(data5, 0),
		},
		{
			name: "Should return distinct values when the in-stream has repeat values",
			fields: fields{
				stream: generateStream(data10),
			},
			want: NewStreamFromSlice(EntrySlice{
				EntryInt(1),
				EntryString("five"),
				EntryString("two"),
				EntryBool(true),
				EntryInt(4),
				EntryBool(false),
			}, 0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stream{
				stream: tt.fields.stream,
			}
			got := s.Distinct().ToSlice()
			assert.EqualValues(t, tt.want.ToSlice(), got)
		})
	}
}

func TestStream_Peek(t *testing.T) {
	var callCount, total int
	computeSumTotal := func(value Entry) {
		callCount++
		total += int(value.(EntryInt))
	}

	type fields struct {
		stream chan Entry
	}
	type args struct {
		consumer Consumer
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		want          Stream
		wantTotal     int
		wantCallCount int
	}{
		{
			name: "Should peek and return empty stream when nil in-stream",
			fields: fields{
				stream: nil,
			},
			args: args{
				consumer: computeSumTotal,
			},
			want:          NewStreamFromSlice(EntrySlice{}, 0),
			wantTotal:     0,
			wantCallCount: 0,
		},
		{
			name: "Should peek and return empty stream when empty in-stream",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() {
						defer close(c)
					}()
					return c
				}(),
			},
			args: args{
				consumer: computeSumTotal,
			},
			want:          NewStreamFromSlice(EntrySlice{}, 0),
			wantTotal:     0,
			wantCallCount: 0,
		},
		{
			name: "Should peek and return stream when populated in-stream",
			fields: fields{
				stream: func() chan Entry {
					c := make(chan Entry)
					go func() {
						defer close(c)
						c <- EntryInt(1)
						c <- EntryInt(2)
						c <- EntryInt(3)
						c <- EntryInt(5)
						c <- EntryInt(8)
					}()
					return c
				}(),
			},
			args: args{
				consumer: computeSumTotal,
			},
			want: NewStreamFromSlice(
				EntrySlice{
					EntryInt(1),
					EntryInt(2),
					EntryInt(3),
					EntryInt(5),
					EntryInt(8),
				}, 0),
			wantTotal:     19,
			wantCallCount: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount, total = 0, 0
			s := Stream{
				stream: tt.fields.stream,
			}
			got := s.Peek(tt.args.consumer)
			assert.EqualValues(t, tt.want.ToSlice(), got.ToSlice())
			assert.Equal(t, tt.wantTotal, total)
			assert.Equal(t, tt.wantCallCount, callCount)
		})
	}
}
