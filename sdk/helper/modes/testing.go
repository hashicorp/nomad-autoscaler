package modes

var TestModes = map[string]string{
	"ent":    "Nomad Autoscaler Enterprise",
	"pro":    "Nomad Autoscaler Pro",
	"expert": "Nomad Autoscaler Expert",
}

type TestStruct struct {
	TopLevelNone        bool `hcl:"top_level_none"`
	TopLevelEnt         bool `hcl:"top_level_ent" modes:"ent"`
	TopLevelExpert      bool `hcl:"top_level_expert" modes:"expert"`
	TopLevelEntOrExpert bool `hcl:"top_level_ent_expert" modes:"ent,expert"`
	TopLevelPro         bool `hcl:"top_level_pro" modes:"pro"`

	NestedNone      *TestNestedStruct `hcl:"nested_none,block"`
	NestedPro       *TestNestedStruct `hcl:"nested_pro,block" modes:"pro"`
	NestedProExpert *TestNestedStruct `hcl:"nested_pro_expert,block" modes:"pro,expert"`

	NonPointerNestedEnt TestNestedStruct `hcl:"non_pointer_nested_ent,block" modes:"ent"`

	NestedMultiple []*TestNestedStruct `hcl:"nested_multiple,block"`
}

type TestNestedStruct struct {
	NestedField    bool `hcl:"nested_field_none"`
	NestedFieldEnt bool `hcl:"nested_field_ent" modes:"ent"`

	DeepNested *TestDeepNested `hcl:"deep_nested,block"`
}

type TestDeepNested struct {
	DeepNestedPro bool `hcl:"deep_nested_pro" modes:"pro"`
}

func NewTestStruct() *TestStruct {
	return &TestStruct{
		NestedNone: &TestNestedStruct{
			DeepNested: &TestDeepNested{},
		},
		NestedPro: &TestNestedStruct{
			DeepNested: &TestDeepNested{},
		},
		NestedProExpert: &TestNestedStruct{
			DeepNested: &TestDeepNested{},
		},
		NonPointerNestedEnt: TestNestedStruct{
			DeepNested: &TestDeepNested{},
		},
		NestedMultiple: []*TestNestedStruct{
			{DeepNested: &TestDeepNested{}},
			{DeepNested: &TestDeepNested{}},
		},
	}
}

func NewTestStructFull() *TestStruct {
	return &TestStruct{
		TopLevelNone:        true,
		TopLevelEnt:         true,
		TopLevelExpert:      true,
		TopLevelEntOrExpert: true,
		TopLevelPro:         true,

		NestedNone: &TestNestedStruct{
			NestedField:    true,
			NestedFieldEnt: true,
			DeepNested: &TestDeepNested{
				DeepNestedPro: true,
			},
		},

		NestedPro: &TestNestedStruct{
			NestedField:    true,
			NestedFieldEnt: true,
			DeepNested: &TestDeepNested{
				DeepNestedPro: true,
			},
		},

		NestedProExpert: &TestNestedStruct{
			NestedField:    true,
			NestedFieldEnt: true,
			DeepNested: &TestDeepNested{
				DeepNestedPro: true,
			},
		},

		NonPointerNestedEnt: TestNestedStruct{
			NestedField:    true,
			NestedFieldEnt: true,
			DeepNested: &TestDeepNested{
				DeepNestedPro: true,
			},
		},

		NestedMultiple: []*TestNestedStruct{
			{DeepNested: &TestDeepNested{DeepNestedPro: true}},
			{DeepNested: &TestDeepNested{DeepNestedPro: true}},
		},
	}
}
