//go:generate stringer -linecomment -type=QType,MType,SelType,FieldType,SkipType,PagingType,AggregrateOp,ValType,ExpOp -output=./gen_string.go
package qcode

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/dosco/graphjin/v2/core/internal/graph"
	"github.com/dosco/graphjin/v2/core/internal/sdata"
	"github.com/dosco/graphjin/v2/core/internal/util"
	plugin "github.com/dosco/graphjin/v2/plugin"
	"github.com/gobuffalo/flect"
)

const (
	maxSelectors   = 100
	singularSuffix = "ByID"
)

type QType int8

const (
	QTUnknown      QType = iota // Unknown
	QTQuery                     // Query
	QTSubscription              // Subcription
	QTMutation                  // Mutation
	QTInsert                    // Insert
	QTUpdate                    // Update
	QTDelete                    // Delete
	QTUpsert                    // Upsert
)

type SelType int8

const (
	SelTypeNone SelType = iota
	SelTypeUnion
	SelTypeMember
)

type SkipType int8

const (
	SkipTypeNone SkipType = iota
	SkipTypeDrop
	SkipTypeUserNeeded
	SkipTypeBlocked
	SkipTypeRemote
)

type ColKey struct {
	Name string
	Base bool
}

type QCode struct {
	Type       QType
	SType      QType
	Name       string
	ActionVar  string
	ActionArg  graph.Arg
	Selects    []Select
	Vars       Variables
	Consts     Constraints
	Roots      []int32
	rootsA     [5]int32
	Mutates    []Mutate
	MUnions    map[string][]int32
	Schema     *sdata.DBSchema
	Remotes    int32
	Cache      Cache
	Script     Script
	Validation Validation
	Typename   bool
}

type Select struct {
	ID         int32
	ParentID   int32
	Type       SelType
	Singular   bool
	Typename   bool
	Table      string
	Schema     string
	FieldName  string
	Fields     []Field
	BCols      []Column
	IArgs      []Arg
	Args       []Arg
	Funcs      []Function
	Where      Filter
	OrderBy    []OrderBy
	DistinctOn []sdata.DBColumn
	GroupCols  bool
	Paging     Paging
	Children   []int32
	SkipRender SkipType
	Ti         sdata.DBTable
	Rel        sdata.DBRel
	Joins      []Join
	order      Order
	through    string
	tc         TConfig
}

type Validation struct {
	Exists bool
	Source string
	Type   string
	VE     plugin.ValidationExecuter
}

type Script struct {
	Exists bool
	Source string
	Name   string
	SC     plugin.ScriptExecuter
}

type TableInfo struct {
	sdata.DBTable
}

type FieldType int8

const (
	FieldTypeCol FieldType = iota
	FieldTypeFunc
)

type Field struct {
	Type        FieldType
	Col         sdata.DBColumn
	Func        sdata.DBFunction
	FieldName   string
	FieldFilter Filter
	Args        []Arg
	SkipRender  SkipType
}

type Column struct {
	Col         sdata.DBColumn
	FieldFilter Filter
	FieldName   string
}

type Function struct {
	Name string
	// Col       sdata.DBColumn
	Func      sdata.DBFunction
	FieldName string
	Alias     string
	Args      []Arg
	Agg       bool
}

type Filter struct {
	*Exp
}

type Exp struct {
	Op    ExpOp
	Joins []Join
	Order
	OrderBy bool

	Left struct {
		ID    int32
		Table string
		Col   sdata.DBColumn
	}
	Right struct {
		ValType  ValType
		Val      string
		ID       int32
		Table    string
		Col      sdata.DBColumn
		ListType ValType
		ListVal  []string
		Path     []string
	}
	Children  []*Exp
	childrenA [5]*Exp
}

type Join struct {
	Filter *Exp
	Rel    sdata.DBRel
	Local  bool
}

type ArgType int8

const (
	ArgTypeVal ArgType = iota
	ArgTypeVar
	ArgTypeCol
)

type Arg struct {
	Type  ArgType
	DType string
	Name  string
	Val   string
	Col   sdata.DBColumn
}

type OrderBy struct {
	KeyVar string
	Key    string
	Col    sdata.DBColumn
	Var    string
	Order  Order
}

type PagingType int8

const (
	PTOffset PagingType = iota
	PTForward
	PTBackward
)

type Paging struct {
	Type      PagingType
	LimitVar  string
	Limit     int32
	OffsetVar string
	Offset    int32
	Cursor    bool
	NoLimit   bool
}

type Cache struct {
	Header string
}

type Variables map[string]json.RawMessage
type Constraints map[string]interface{}

type ExpOp int8

const (
	OpNop ExpOp = iota
	OpAnd
	OpOr
	OpNot
	OpEquals
	OpNotEquals
	OpGreaterOrEquals
	OpLesserOrEquals
	OpGreaterThan
	OpLesserThan
	OpIn
	OpNotIn
	OpLike
	OpNotLike
	OpILike
	OpNotILike
	OpSimilar
	OpNotSimilar
	OpRegex
	OpNotRegex
	OpIRegex
	OpNotIRegex
	OpContains
	OpContainedIn
	OpHasInCommon
	OpHasKey
	OpHasKeyAny
	OpHasKeyAll
	OpIsNull
	OpIsNotNull
	OpTsQuery
	OpFalse
	OpNotDistinct
	OpDistinct
	OpEqualsTrue
	OpNotEqualsTrue
	OpSelectExists
)

type ValType int8

const (
	ValStr ValType = iota + 1
	ValNum
	ValBool
	ValList
	ValObj
	ValVar
)

type AggregrateOp int8

const (
	AgCount AggregrateOp = iota + 1
	AgSum
	AgAvg
	AgMax
	AgMin
)

type Order int8

const (
	OrderNone Order = iota
	OrderAsc
	OrderDesc
	OrderAscNullsFirst
	OrderAscNullsLast
	OrderDescNullsFirst
	OrderDescNullsLast
)

func (o Order) String() string {
	return []string{"None", "ASC", "DESC", "ASC NULLS FIRST", "ASC NULLS LAST", "DESC NULLLS FIRST", "DESC NULLS LAST"}[o]
}

type Compiler struct {
	c  Config
	s  *sdata.DBSchema
	tr map[string]trval
}

func NewCompiler(s *sdata.DBSchema, c Config) (*Compiler, error) {
	if c.DBSchema == "" {
		c.DBSchema = "public"
	}

	c.defTrv.query.block = c.DefaultBlock
	c.defTrv.insert.block = c.DefaultBlock
	c.defTrv.update.block = c.DefaultBlock
	c.defTrv.upsert.block = c.DefaultBlock
	c.defTrv.delete.block = c.DefaultBlock

	return &Compiler{c: c, s: s, tr: make(map[string]trval)}, nil
}

func (co *Compiler) Compile(
	query []byte, vars Variables, role, namespace string) (*QCode, error) {
	var err error

	op, err := graph.Parse(query)
	if err != nil {
		return nil, err
	}

	qc := QCode{Name: op.Name, SType: QTQuery, Schema: co.s, Vars: vars}
	qc.Roots = qc.rootsA[:0]
	qc.Type = GetQType(op.Type)

	if err := co.compileQuery(&qc, &op, role); err != nil {
		return nil, err
	}

	if qc.Type == QTMutation {
		if err := co.compileMutation(&qc, role); err != nil {
			return nil, err
		}
	}

	return &qc, nil
}

func (co *Compiler) compileQuery(qc *QCode, op *graph.Operation, role string) error {
	var id int32

	if len(op.Fields) == 0 {
		return errors.New("invalid graphql no query found")
	}

	if op.Type == graph.OpMutate {
		if err := co.setMutationType(qc, op, role); err != nil {
			return err
		}
	}
	if err := co.compileOpDirectives(qc, op.Directives); err != nil {
		return err
	}

	qc.Selects = make([]Select, 0, 5)
	st := util.NewStackInt32()

	if len(op.Fields) == 0 {
		return errors.New("empty query")
	}

	for _, f := range op.Fields {
		if f.ParentID == -1 {
			if f.Name == "__typename" && op.Name != "" {
				qc.Typename = true
			}
			val := f.ID | (-1 << 16)
			st.Push(val)
		}
	}

	for {
		if st.Len() == 0 {
			break
		}

		if id >= maxSelectors {
			return fmt.Errorf("selector limit reached (%d)", maxSelectors)
		}

		val := st.Pop()
		fid := val & 0xFFFF
		parentID := (val >> 16) & 0xFFFF

		field := op.Fields[fid]

		// A keyword is a cursor field at the top-level
		// For example posts_cursor in the root
		if field.Type == graph.FieldKeyword {
			continue
		}

		if field.ParentID == -1 {
			parentID = -1
		}

		s1 := Select{
			ID:       id,
			ParentID: parentID,
		}
		sel := &s1

		if co.c.EnableCamelcase {
			if field.Alias == "" {
				field.Alias = field.Name
			}
			field.Name = util.ToSnake(field.Name)
		}

		if field.Alias != "" {
			sel.FieldName = field.Alias
		} else {
			sel.FieldName = field.Name
		}

		sel.Children = make([]int32, 0, 5)

		if err := co.compileSelectorDirectives1(qc, sel, field.Directives, role); err != nil {
			return err
		}

		if err := co.addRelInfo(op, qc, sel, field); err != nil {
			return err
		}

		if err := co.compileSelectorDirectives2(qc, sel, field.Directives, role); err != nil {
			return err
		}

		tr, err := co.setSelectorRoleConfig(role, field.Name, qc, sel)
		if err != nil {
			return err
		}

		co.setLimit(tr, qc, sel)

		if err := co.compileArgs(sel, field.Args, role); err != nil {
			return err
		}

		if err := co.compileFields(st, op, qc, sel, field, tr, role); err != nil {
			return err
		}

		// Order is important AddFilters must come after compileArgs
		if userNeeded := addFilters(qc, &sel.Where, tr); userNeeded && role == "anon" {
			sel.SkipRender = SkipTypeUserNeeded
		}

		// If an actual cursor is available
		if sel.Paging.Cursor {
			// Set tie-breaker order column for the cursor direction
			// this column needs to be the last in the order series.
			if err := co.orderByIDCol(sel); err != nil {
				return err
			}

			// Set filter chain needed to make the cursor work
			if sel.Paging.Type != PTOffset {
				co.addSeekPredicate(sel)
			}
		}

		// Compute and set the relevant where clause required to join
		// this table with its parent
		co.setRelFilters(qc, sel)

		if err := co.validateSelect(sel); err != nil {
			return err
		}

		qc.Selects = append(qc.Selects, s1)
		id++
	}

	if id == 0 {
		return errors.New("invalid query: no selectors found")
	}

	return nil
}

func (co *Compiler) addRelInfo(
	op *graph.Operation, qc *QCode, sel *Select, field graph.Field) error {
	var psel *Select
	var childF, parentF graph.Field
	var err error

	childF = field

	if sel.ParentID == -1 {
		qc.Roots = append(qc.Roots, sel.ID)
	} else {
		psel = &qc.Selects[sel.ParentID]
		psel.Children = append(psel.Children, sel.ID)
		parentF = op.Fields[field.ParentID]
	}

	switch field.Type {
	case graph.FieldUnion:
		sel.Type = SelTypeUnion
		if psel == nil {
			return fmt.Errorf("union types are only valid with polymorphic relationships")
		}

	case graph.FieldMember:
		// TODO: Fix this
		// if sel.Table != sel.Table {
		// 	return fmt.Errorf("inline fragment: 'on %s' should be 'on %s'", sel.Table, sel.Table)
		// }
		sel.Type = SelTypeMember
		sel.Singular = psel.Singular

		childF = parentF
		parentF = op.Fields[int(parentF.ParentID)]
	}

	if sel.Rel.Type == sdata.RelSkip {
		sel.Rel.Type = sdata.RelNone

	} else if sel.ParentID != -1 {
		if co.c.EnableCamelcase {
			parentF.Name = util.ToSnake(parentF.Name)
		}
		path, err := co.FindPath(childF.Name, parentF.Name, sel.through)
		if err != nil {
			return graphError(err, childF.Name, parentF.Name, sel.through)
		}
		sel.Rel = sdata.PathToRel(path[0])

		// for _, p := range path {
		// 	rel := sdata.PathToRel(p)
		// 	fmt.Println(childF.Name, parentF.Name,
		// 		"--->>>", rel.Left.Col.Table, rel.Left.Col.Name,
		// 		"|", rel.Right.Col.Table, rel.Right.Col.Name)
		// }

		rpath := path[1:]

		for i := len(rpath) - 1; i >= 0; i-- {
			p := rpath[i]
			rel := sdata.PathToRel(p)
			var pid int32
			if i == len(rpath)-1 {
				pid = sel.ParentID
			} else {
				pid = -1
			}
			sel.Joins = append(sel.Joins, Join{
				Rel:    rel,
				Filter: buildFilter(rel, pid),
			})
		}
	}

	if sel.ParentID == -1 ||
		sel.Rel.Type == sdata.RelPolymorphic ||
		sel.Rel.Type == sdata.RelNone {
		schema := co.c.DBSchema
		if sel.Schema != "" {
			schema = sel.Schema
		}
		if sel.Ti, err = co.Find(schema, field.Name); err != nil {
			return err
		}
	} else {
		sel.Ti = sel.Rel.Left.Ti
	}

	if sel.Ti.Blocked {
		return fmt.Errorf("table: '%t' (%s) blocked", sel.Ti.Blocked, field.Name)
	}

	sel.Table = sel.Ti.Name
	sel.tc = co.getTConfig(sel.Ti.Schema, sel.Ti.Name)

	if sel.Rel.Type == sdata.RelRemote {
		sel.Table = field.Name
		qc.Remotes++
		return nil
	}

	co.setSingular(field.Name, sel)
	return nil
}

func (co *Compiler) setRelFilters(qc *QCode, sel *Select) {
	rel := sel.Rel
	pid := sel.ParentID

	if len(sel.Joins) != 0 {
		pid = -1
	}

	switch rel.Type {
	case sdata.RelOneToOne, sdata.RelOneToMany:
		setFilter(&sel.Where, buildFilter(rel, pid))

	case sdata.RelEmbedded:
		setFilter(&sel.Where, buildFilter(rel, pid))

	case sdata.RelPolymorphic:
		pid = qc.Selects[sel.ParentID].ParentID
		ex := newExpOp(OpAnd)

		ex1 := newExpOp(OpEquals)
		ex1.Left.Table = sel.Ti.Name
		ex1.Left.Col = rel.Right.Col
		ex1.Right.ID = pid
		ex1.Right.Col = rel.Left.Col

		ex2 := newExpOp(OpEquals)
		ex2.Left.ID = pid
		ex2.Left.Col.Table = rel.Left.Col.Table
		ex2.Left.Col.Name = rel.Left.Col.FKeyCol
		ex2.Right.ValType = ValStr
		ex2.Right.Val = sel.Ti.Name

		ex.Children = []*Exp{ex1, ex2}
		setFilter(&sel.Where, ex)

	case sdata.RelRecursive:
		rcte := "__rcte_" + rel.Right.Ti.Name
		ex := newExpOp(OpAnd)
		ex1 := newExpOp(OpIsNotNull)
		ex2 := newExp()
		ex3 := newExp()

		v, _ := sel.GetInternalArg("find")
		switch v.Val {
		case "parents", "parent":
			ex1.Left.Table = rcte
			ex1.Left.Col = rel.Left.Col
			switch {
			case !rel.Left.Col.Array && rel.Right.Col.Array:
				ex2.Op = OpNotIn
				ex2.Left.Table = rcte
				ex2.Left.Col = rel.Left.Col
				ex2.Right.Table = rcte
				ex2.Right.Col = rel.Right.Col

				ex3.Op = OpIn
				ex3.Left.Table = rcte
				ex3.Left.Col = rel.Left.Col
				ex3.Right.Col = rel.Right.Col

			case rel.Left.Col.Array && !rel.Right.Col.Array:
				ex2.Op = OpNotIn
				ex2.Left.Table = rcte
				ex2.Left.Col = rel.Right.Col
				ex2.Right.Table = rcte
				ex2.Right.Col = rel.Left.Col

				ex3.Op = OpIn
				ex3.Left.Col = rel.Right.Col
				ex3.Right.Table = rcte
				ex3.Right.Col = rel.Left.Col

			default:
				ex2.Op = OpNotEquals
				ex2.Left.Table = rcte
				ex2.Left.Col = rel.Left.Col
				ex2.Right.Table = rcte
				ex2.Right.Col = rel.Right.Col

				ex3.Op = OpEquals
				ex3.Left.Col = rel.Right.Col
				ex3.Right.Table = rcte
				ex3.Right.Col = rel.Left.Col
			}

		default:
			ex1.Left.Col = rel.Left.Col
			switch {
			case !rel.Left.Col.Array && rel.Right.Col.Array:
				ex2.Op = OpNotIn
				ex2.Left.Col = rel.Left.Col
				ex2.Right.Col = rel.Right.Col

				ex3.Op = OpIn
				ex3.Left.Col = rel.Left.Col
				ex3.Right.Table = rcte
				ex3.Right.Col = rel.Right.Col

			case rel.Left.Col.Array && !rel.Right.Col.Array:
				ex2.Op = OpNotIn
				ex2.Left.Col = rel.Right.Col
				ex2.Right.Col = rel.Left.Col

				ex3.Op = OpIn
				ex3.Left.Table = rcte
				ex3.Left.Col = rel.Right.Col
				ex3.Right.Col = rel.Left.Col

			default:
				ex2.Op = OpNotEquals
				ex2.Left.Col = rel.Left.Col
				ex2.Right.Col = rel.Right.Col

				ex3.Op = OpEquals
				ex3.Left.Col = rel.Left.Col
				ex3.Right.Table = rcte
				ex3.Right.Col = rel.Right.Col
			}
		}

		ex.Children = []*Exp{ex1, ex2, ex3}
		setFilter(&sel.Where, ex)
	}
}

func (co *Compiler) Find(schema, name string) (sdata.DBTable, error) {
	name = strings.TrimSuffix(name, singularSuffix)
	return co.s.Find(schema, name)
}

func (co *Compiler) FindPath(from, to, through string) ([]sdata.TPath, error) {
	from = strings.TrimSuffix(from, singularSuffix)
	to = strings.TrimSuffix(to, singularSuffix)
	return co.s.FindPath(from, to, through)
}

func buildFilter(rel sdata.DBRel, pid int32) *Exp {
	switch rel.Type {
	case sdata.RelOneToOne, sdata.RelOneToMany:
		ex := newExp()
		switch {
		case !rel.Left.Col.Array && rel.Right.Col.Array:
			ex.Op = OpIn
			ex.Left.Col = rel.Left.Col
			ex.Right.ID = pid
			ex.Right.Col = rel.Right.Col

		case rel.Left.Col.Array && !rel.Right.Col.Array:
			ex.Op = OpIn
			ex.Left.ID = pid
			ex.Left.Col = rel.Right.Col
			ex.Right.Col = rel.Left.Col

		default:
			ex.Op = OpEquals
			ex.Left.Col = rel.Left.Col
			ex.Right.ID = pid
			ex.Right.Col = rel.Right.Col
		}
		return ex

	case sdata.RelEmbedded:
		ex := newExpOp(OpEquals)
		ex.Left.Col = rel.Right.Col
		ex.Right.ID = pid
		ex.Right.Col = rel.Right.Col
		return ex

	default:
		return nil
	}
}

func (co *Compiler) setSingular(fieldName string, sel *Select) {
	if sel.Singular {
		return
	}

	if co.c.EnableInflection {
		sel.Singular = (flect.Singularize(fieldName) == fieldName)
	}

	if len(sel.Joins) != 0 {
		return
	}

	if (sel.Rel.Type == sdata.RelOneToMany && !sel.Rel.Right.Col.Array) ||
		sel.Rel.Type == sdata.RelPolymorphic {
		sel.Singular = true
		return
	}
}

func (co *Compiler) setSelectorRoleConfig(role, fieldName string, qc *QCode, sel *Select) (trval, error) {
	tr := co.getRole(role, sel.Ti.Schema, sel.Ti.Name, fieldName)

	if tr.isBlocked(qc.SType) {
		if qc.SType != QTQuery {
			return tr, fmt.Errorf("%s blocked: %s (role: %s)", qc.SType, fieldName, role)
		}
		sel.SkipRender = SkipTypeBlocked
	}
	return tr, nil
}

func (co *Compiler) setLimit(tr trval, qc *QCode, sel *Select) {
	if sel.Paging.Limit != 0 {
		return
	}
	// Use limit from table role config
	if l := tr.limit(qc.Type); l != 0 {
		sel.Paging.Limit = l

		// Else use default limit from config
	} else if co.c.DefaultLimit != 0 {
		sel.Paging.Limit = int32(co.c.DefaultLimit)

		// Else just go with 20
	} else {
		sel.Paging.Limit = 20
	}
}

// This
// (A, B, C) >= (X, Y, Z)
//
// Becomes
// (A > X)
//   OR ((A = X) AND (B > Y))
//   OR ((A = X) AND (B = Y) AND (C > Z))
//   OR ((A = X) AND (B = Y) AND (C = Z)

func (co *Compiler) addSeekPredicate(sel *Select) {
	var or, and *Exp
	obLen := len(sel.OrderBy)

	if obLen != 0 {
		or = newExpOp(OpOr)

		isnull := newExpOp(OpIsNull)
		isnull.Left.Table = "__cur"
		isnull.Left.Col = sel.OrderBy[0].Col

		or.Children = []*Exp{isnull}
	}

	for i := 0; i < obLen; i++ {
		if i != 0 {
			and = newExpOp(OpAnd)
		}

		for n, ob := range sel.OrderBy {
			if n > i {
				break
			}

			f := newExp()
			f.Left.Col = ob.Col
			f.Right.Table = "__cur"
			f.Right.Col = ob.Col

			switch {
			case i > 0 && n != i:
				f.Op = OpEquals
			case ob.Order == OrderDesc:
				f.Op = OpLesserThan
			default:
				f.Op = OpGreaterThan
			}

			if and != nil {
				and.Children = append(and.Children, f)
			} else {
				or.Children = append(or.Children, f)
			}
		}

		if and != nil {
			or.Children = append(or.Children, and)
		}
	}

	setFilter(&sel.Where, or)
}

func addFilters(qc *QCode, where *Filter, trv trval) bool {
	if fil, userNeeded := trv.filter(qc.SType); fil != nil {
		switch fil.Op {
		case OpNop:
		case OpFalse:
			where.Exp = fil
		default:
			setFilter(where, fil)
		}
		return userNeeded
	}

	return false
}

func (co *Compiler) compileOpDirectives(qc *QCode, dirs []graph.Directive) error {
	var err error

	for i := range dirs {
		d := &dirs[i]

		switch d.Name {
		case "cacheControl":
			err = co.compileDirectiveCacheControl(qc, d)

		case "script":
			err = co.compileDirectiveScript(qc, d)

		case "constraint", "validate":
			err = co.compileDirectiveConstraint(qc, d)

		case "validation":
			err = co.compileDirectiveValidation(qc, d)

		default:
			err = fmt.Errorf("unknown operation level directive: %s", d.Name)
		}

		if err != nil {
			return err
		}
	}
	return nil
}

func (co *Compiler) compileFieldDirectives(sel *Select, f *Field, dirs []graph.Directive, role string) error {
	var err error

	for i := range dirs {
		d := &dirs[i]

		switch d.Name {
		case "skip":
			err = co.compileFieldDirectiveSkipInclude(true, sel, f, d, role)

		case "include":
			err = co.compileFieldDirectiveSkipInclude(false, sel, f, d, role)

		default:
			err = fmt.Errorf("unknown field level directive: %s", d.Name)
		}

		if err != nil {
			return err
		}
	}
	return nil
}

// these directives need to run before the relationship resolution code
func (co *Compiler) compileSelectorDirectives1(qc *QCode, sel *Select, dirs []graph.Directive, role string) error {
	var err error

	for i := range dirs {
		d := &dirs[i]

		switch d.Name {
		case "schema":
			err = co.compileDirectiveSchema(sel, d)

		case "notRelated", "not_related":
			err = co.compileDirectiveNotRelated(sel, d)

		case "through":
			err = co.compileDirectiveThrough(sel, d)
		}

		if err != nil {
			return fmt.Errorf("directive @%s: %w", d.Name, err)
		}
	}

	return nil
}

func (co *Compiler) compileSelectorDirectives2(qc *QCode, sel *Select, dirs []graph.Directive, role string) error {
	var err error

	for i := range dirs {
		d := &dirs[i]

		switch d.Name {
		case "schema", "through", "notRelated", "not_related":

		case "skip":
			err = co.compileSelectDirectiveSkipInclude(true, sel, d, role)

		case "include":
			err = co.compileSelectDirectiveSkipInclude(false, sel, d, role)

		case "object":
			sel.Singular = true
			sel.Paging.Limit = 1

		default:
			err = fmt.Errorf("no such selector level directive")
		}

		if err != nil {
			return fmt.Errorf("directive @%s: %w", d.Name, err)
		}
	}

	return nil
}

func (co *Compiler) compileArgs(sel *Select, args []graph.Arg, role string) error {
	var err error

	for i := range args {
		arg := &args[i]

		switch arg.Name {
		case "id":
			err = co.compileArgID(sel, arg)

		case "search":
			err = co.compileArgSearch(sel, arg)

		case "where":
			err = co.compileArgWhere(sel, arg, role)

		case "orderby", "order_by", "order":
			err = co.compileArgOrderBy(sel, arg)

		case "distinct_on", "distinct":
			err = co.compileArgDistinctOn(sel, arg)

		case "limit":
			err = co.compileArgLimit(sel, arg)

		case "offset":
			err = co.compileArgOffset(sel, arg)

		case "first":
			err = co.compileArgFirstLast(sel, arg, OrderAsc)

		case "last":
			err = co.compileArgFirstLast(sel, arg, OrderDesc)

		case "after":
			err = co.compileArgAfterBefore(sel, arg, PTForward)

		case "before":
			err = co.compileArgAfterBefore(sel, arg, PTBackward)

		case "find":
			err = co.compileArgFind(sel, arg)

		case "args":
			err = co.compileArgArgs(sel, arg)

		default:
			if sel.Ti.Type == "function" {
				err = co.compileFuncTableArg(sel, arg)
			}
		}

		if err != nil {
			return fmt.Errorf("%s: %w", arg.Name, err)
		}
	}

	return nil
}

func (co *Compiler) validateSelect(sel *Select) error {
	if sel.Rel.Type == sdata.RelRecursive {
		v, ok := sel.GetInternalArg("find")
		if !ok {
			return fmt.Errorf("argument 'find' needed for recursive queries")
		}
		if v.Val != "parents" && v.Val != "children" {
			return fmt.Errorf("valid values for 'find' are 'parents' and 'children'")
		}
	}
	return nil
}

func (co *Compiler) setMutationType(qc *QCode, op *graph.Operation, role string) error {
	var err error

	setActionVar := func(arg graph.Arg) error {
		v := arg.Val
		if v.Type != graph.NodeVar &&
			v.Type != graph.NodeObj &&
			(v.Type != graph.NodeList || len(v.Children) == 0 && v.Children[0].Type != graph.NodeObj) {
			return argErr(arg.Name, "variable, an object or a list of objects")
		}
		qc.ActionVar = arg.Val.Val
		qc.ActionArg = arg
		return nil
	}

	args := op.Fields[0].Args

	for _, arg := range args {
		switch arg.Name {
		case "insert":
			qc.SType = QTInsert
			err = setActionVar(arg)
		case "update":
			qc.SType = QTUpdate
			err = setActionVar(arg)
		case "upsert":
			qc.SType = QTUpsert
			err = setActionVar(arg)
		case "delete":
			qc.SType = QTDelete
			if ifNotArg(arg, graph.NodeBool) || ifNotArgVal(arg, "true") {
				err = errors.New("value for 'delete' must be 'true'")
			}
		}

		if err != nil {
			return err
		}
	}

	if qc.SType == QTUnknown {
		return errors.New(`mutations must contains one of the following arguments (insert, update, upsert or delete)`)
	}

	return nil
}

func (co *Compiler) compileDirectiveSchema(sel *Select, d *graph.Directive) error {
	if len(d.Args) == 0 {
		return fmt.Errorf("required argument 'name' missing")
	}
	arg := d.Args[0]

	if ifNotArg(arg, graph.NodeStr) {
		return argTypeErr("string")
	}

	sel.Schema = arg.Val.Val
	return nil
}

func (co *Compiler) compileSelectDirectiveSkipInclude(skip bool, sel *Select, d *graph.Directive, role string) (err error) {
	err, drop := co.compileSkipInclude(skip, sel, -1, &sel.Where, d, role)
	if err != nil {
		return err
	}
	if drop {
		sel.SkipRender = SkipTypeDrop
	}
	return
}

func (co *Compiler) compileFieldDirectiveSkipInclude(skip bool, sel *Select, f *Field, d *graph.Directive, role string) (err error) {
	var drop bool
	if f.Type == FieldTypeFunc {
		err, drop = co.compileSkipInclude(skip, sel, -1, &f.FieldFilter, d, role)
	} else {
		err, drop = co.compileSkipInclude(skip, sel, sel.ID, &f.FieldFilter, d, role)
	}
	if err != nil {
		return err
	}
	if drop {
		f.SkipRender = SkipTypeDrop
	}
	return
}

func (co *Compiler) compileSkipInclude(
	skip bool,
	sel *Select,
	selID int32,
	fil *Filter,
	d *graph.Directive,
	role string) (err error, drop bool) {

	if len(d.Args) == 0 {
		err = fmt.Errorf("arguments 'if' or 'if_role' expected")
		return
	}

	for _, arg := range d.Args {
		switch arg.Name {
		case "if":
			err = co.compileSkipIncludeFilter(
				skip, sel, selID, fil, arg, role)
			if err != nil {
				return
			}
		case "if_role", "ifRole":
			if ifArg(arg, graph.NodeStr) {
				switch {
				case skip && arg.Val.Val == role:
					drop = true
				case !skip && arg.Val.Val != role:
					drop = true
				}
				return
			}
			err = argErr(arg.Name, "string")
			return

		default:
			err = fmt.Errorf("invalid argument: %s", arg.Name)
			return
		}
	}
	return
}

func (co *Compiler) compileSkipIncludeFilter(
	skip bool,
	sel *Select,
	selID int32,
	fil *Filter,
	arg graph.Arg,
	role string) error {

	if ifArg(arg, graph.NodeVar) {
		var ex *Exp
		if skip {
			ex = newExpOp(OpNotEqualsTrue)
		} else {
			ex = newExpOp(OpEqualsTrue)
		}
		ex.Right.ValType = ValVar
		ex.Right.Val = arg.Val.Val
		setFilter(fil, ex)
		return nil
	}

	if ifArg(arg, graph.NodeObj) {
		if skip {
			setFilter(fil, newExpOp(OpNot))
		}
		return co.compileAndSetFilter(sel, selID, fil, &arg, role)
	}
	return argErr("if", "variable or filter expression")
}

func (co *Compiler) compileDirectiveCacheControl(qc *QCode, d *graph.Directive) error {
	var maxAge string
	var scope string

	for _, arg := range d.Args {
		switch arg.Name {
		case "maxAge":
			if ifNotArg(arg, graph.NodeNum) {
				return argErr("maxAge", "number")
			}
			maxAge = arg.Val.Val
		case "scope":
			if ifNotArg(arg, graph.NodeStr) {
				return argErr("scope", "string")
			}
			scope = arg.Val.Val
		default:
			return fmt.Errorf("invalid argument: %s", arg.Name)
		}
	}

	if len(d.Args) == 0 || maxAge == "" {
		return fmt.Errorf("required argument 'maxAge' missing")
	}

	hdr := []string{"max-age=" + maxAge}

	if scope != "" {
		hdr = append(hdr, scope)
	}

	qc.Cache.Header = strings.Join(hdr, " ")
	return nil
}

func (co *Compiler) compileDirectiveScript(qc *QCode, d *graph.Directive) error {
	if len(d.Args) == 0 {
		return argErr("name", "string")
	}

	if d.Args[0].Name == "name" {
		if ifNotArg(d.Args[0], graph.NodeStr) {
			return argErr("name", "string")
		}
		qc.Script.Name = d.Args[0].Val.Val
	}

	if qc.Script.Name == "" {
		qc.Script.Name = qc.Name
	}

	if qc.Script.Name == "" {
		return fmt.Errorf("required argument 'name' missing")
	}

	if path.Ext(qc.Script.Name) == "" {
		qc.Script.Name += ".js"
	}

	return nil
}

type validator struct {
	name   string
	types  []graph.ParserType
	single bool
}

var validators = map[string]validator{
	"variable":                 {name: "variable", types: []graph.ParserType{graph.NodeStr}},
	"error":                    {name: "error", types: []graph.ParserType{graph.NodeStr}},
	"unique":                   {name: "unique", types: []graph.ParserType{graph.NodeBool}, single: true},
	"format":                   {name: "format", types: []graph.ParserType{graph.NodeStr}, single: true},
	"required":                 {name: "required", types: []graph.ParserType{graph.NodeBool}, single: true},
	"requiredIf":               {name: "required_if", types: []graph.ParserType{graph.NodeObj}},
	"requiredUnless":           {name: "required_unless", types: []graph.ParserType{graph.NodeObj}},
	"requiredWith":             {name: "required_with", types: []graph.ParserType{graph.NodeList, graph.NodeStr}},
	"requiredWithAll":          {name: "required_with_all", types: []graph.ParserType{graph.NodeList, graph.NodeStr}},
	"requiredWithout":          {name: "required_without", types: []graph.ParserType{graph.NodeList, graph.NodeStr}},
	"requiredWithoutAll":       {name: "required_without_all", types: []graph.ParserType{graph.NodeList, graph.NodeStr}},
	"length":                   {name: "len", types: []graph.ParserType{graph.NodeStr, graph.NodeNum}},
	"max":                      {name: "max", types: []graph.ParserType{graph.NodeStr, graph.NodeNum}},
	"min":                      {name: "min", types: []graph.ParserType{graph.NodeStr, graph.NodeNum}},
	"equals":                   {name: "eq", types: []graph.ParserType{graph.NodeStr, graph.NodeNum}},
	"notEquals":                {name: "neq", types: []graph.ParserType{graph.NodeStr, graph.NodeNum}},
	"oneOf":                    {name: "oneof", types: []graph.ParserType{graph.NodeList, graph.NodeNum, graph.NodeList, graph.NodeStr}},
	"greaterThan":              {name: "gt", types: []graph.ParserType{graph.NodeStr, graph.NodeNum}},
	"greaterThanOrEquals":      {name: "gte", types: []graph.ParserType{graph.NodeStr, graph.NodeNum}},
	"lessThan":                 {name: "lt", types: []graph.ParserType{graph.NodeStr, graph.NodeNum}},
	"lessThanOrEquals":         {name: "lte", types: []graph.ParserType{graph.NodeStr, graph.NodeNum}},
	"equalsField":              {name: "eqfield", types: []graph.ParserType{graph.NodeStr}},
	"notEqualsField":           {name: "nefield", types: []graph.ParserType{graph.NodeStr}},
	"greaterThanField":         {name: "gtfield", types: []graph.ParserType{graph.NodeStr}},
	"greaterThanOrEqualsField": {name: "gtefield", types: []graph.ParserType{graph.NodeStr}},
	"lessThanField":            {name: "ltfield", types: []graph.ParserType{graph.NodeStr}},
	"lessThanOrEqualsField":    {name: "ltefield", types: []graph.ParserType{graph.NodeStr}},
}

func (co *Compiler) compileDirectiveConstraint(qc *QCode, d *graph.Directive) error {
	var varName string
	var errMsg string
	var vals []string

	for _, a := range d.Args {
		if a.Name == "variable" && ifNotArgVal(a, "") {
			if a.Val.Val[0] == '$' {
				varName = a.Val.Val[1:]
			} else {
				varName = a.Val.Val
			}
			continue
		}

		if a.Name == "error" && ifNotArgVal(a, "") {
			errMsg = a.Val.Val
		}

		if a.Name == "format" && ifNotArgVal(a, "") {
			vals = append(vals, a.Val.Val)
			continue
		}

		v, ok := validators[a.Name]
		if !ok {
			continue
		}

		if err := validateConstraint(a, v); err != nil {
			return err
		}

		if v.single {
			vals = append(vals, v.name)
			continue
		}

		var value string
		switch a.Val.Type {
		case graph.NodeStr, graph.NodeNum, graph.NodeBool:
			if ifNotArgVal(a, "") {
				value = a.Val.Val
			}

		case graph.NodeObj:
			var items []string
			for _, v := range a.Val.Children {
				items = append(items, v.Name, v.Val)
			}
			value = strings.Join(items, " ")

		case graph.NodeList:
			var items []string
			for _, v := range a.Val.Children {
				items = append(items, v.Val)
			}
			value = strings.Join(items, " ")
		}

		vals = append(vals, (v.name + "=" + value))
	}

	if varName == "" {
		return errors.New("invalid @constraint no variable name specified")
	}

	if qc.Consts == nil {
		qc.Consts = make(map[string]interface{})
	}

	opt := strings.Join(vals, ",")
	if errMsg != "" {
		opt += "~" + errMsg
	}

	qc.Consts[varName] = opt
	return nil
}

func validateConstraint(a graph.Arg, v validator) error {
	list := false
	for _, t := range v.types {
		switch {
		case t == graph.NodeList:
			list = true
		case list && ifArgList(a, t):
			return nil
		case ifArg(a, t):
			return nil
		}
	}

	list = false
	err := "value must be of type: "

	for i, t := range v.types {
		if i != 0 {
			err += ", "
		}
		if !list && t == graph.NodeList {
			err += "a list of "
			list = true
		}
		err += t.String()
	}
	return errors.New(err)
}

func (co *Compiler) compileDirectiveNotRelated(sel *Select, d *graph.Directive) error {
	sel.Rel.Type = sdata.RelSkip
	return nil
}

func (co *Compiler) compileDirectiveThrough(sel *Select, d *graph.Directive) error {
	if len(d.Args) == 0 {
		return fmt.Errorf("required argument 'table' or 'column'")
	}
	arg := d.Args[0]

	if arg.Name == "table" || arg.Name == "column" {
		if arg.Val.Type != graph.NodeStr {
			return argErr(arg.Name, "string")
		}
		sel.through = arg.Val.Val
	}

	return nil
}
func (co *Compiler) compileDirectiveValidation(qc *QCode, d *graph.Directive) error {
	if len(d.Args) == 0 {
		return fmt.Errorf("required arguments 'src' and 'type'")
	}

	for _, arg := range d.Args {
		switch arg.Name {
		case "src", "source":
			qc.Validation.Source = arg.Val.Val
		case "type", "lang":
			qc.Validation.Type = arg.Val.Val
		default:
			return fmt.Errorf("invalid argument '%s'", arg.Name)
		}
	}

	if qc.Validation.Source == "" {
		return errors.New("validation script not set")
	}

	if qc.Validation.Type == "" {
		return errors.New("validation type not set")
	}

	return nil
}

func (co *Compiler) compileArgFind(sel *Select, arg *graph.Arg) error {
	// Only allow on recursive relationship selectors
	if sel.Rel.Type != sdata.RelRecursive {
		return fmt.Errorf("selector '%s' is not recursive", sel.FieldName)
	}
	if arg.Val.Val != "parents" && arg.Val.Val != "children" {
		return fmt.Errorf("valid values 'parents' or 'children'")
	}
	sel.addIArg(Arg{Name: arg.Name, Val: arg.Val.Val})
	return nil
}

func (co *Compiler) compileArgID(sel *Select, arg *graph.Arg) error {
	node := arg.Val

	if sel.ParentID != -1 {
		return fmt.Errorf("can only be specified at the query root")
	}

	if node.Type != graph.NodeNum &&
		node.Type != graph.NodeStr &&
		node.Type != graph.NodeVar {
		return argTypeErr("number, string or variable")
	}

	if sel.Ti.PrimaryCol.Name == "" {
		return fmt.Errorf("no primary key column defined for '%s'", sel.Table)
	}

	ex := newExpOp(OpEquals)
	ex.Left.Col = sel.Ti.PrimaryCol

	switch node.Type {
	case graph.NodeNum:
		if _, err := strconv.ParseInt(node.Val, 10, 32); err != nil {
			return err
		} else {
			ex.Right.ValType = ValNum
			ex.Right.Val = node.Val
		}

	case graph.NodeStr:
		ex.Right.ValType = ValStr
		ex.Right.Val = node.Val

	case graph.NodeVar:
		ex.Right.ValType = ValVar
		ex.Right.Val = node.Val
	}

	sel.Where.Exp = ex
	sel.Singular = true
	return nil
}

func (co *Compiler) compileArgSearch(sel *Select, arg *graph.Arg) error {
	if len(sel.Ti.FullText) == 0 {
		switch co.s.DBType() {
		case "mysql":
			return fmt.Errorf("no fulltext indexes defined for table '%s'", sel.Table)
		default:
			return fmt.Errorf("no tsvector column defined on table '%s'", sel.Table)
		}
	}

	if arg.Val.Type != graph.NodeVar {
		return argTypeErr("variable")
	}

	ex := newExpOp(OpTsQuery)
	ex.Right.ValType = ValVar
	ex.Right.Val = arg.Val.Val

	sel.addIArg(Arg{Name: arg.Name, Val: arg.Val.Val})
	setFilter(&sel.Where, ex)
	return nil
}

func (co *Compiler) compileArgWhere(sel *Select, arg *graph.Arg, role string) error {
	return co.compileAndSetFilter(sel, -1, &sel.Where, arg, role)
}

func (co *Compiler) compileArgOrderBy(sel *Select, arg *graph.Arg) error {
	node := arg.Val

	if node.Type != graph.NodeObj &&
		node.Type != graph.NodeVar {
		return argTypeErr("object or variable")
	}

	cm := make(map[string]struct{})

	for _, ob := range sel.OrderBy {
		cm[ob.Col.Name] = struct{}{}
	}

	switch node.Type {
	case graph.NodeObj:
		return co.compileArgOrderByObj(sel, node, cm)

	case graph.NodeVar:
		return co.compileArgOrderByVar(sel, node, cm)
	}

	return nil
}

func (co *Compiler) compileArgOrderByObj(sel *Select, parent *graph.Node, cm map[string]struct{}) error {
	st := util.NewStackInf()

	for i := range parent.Children {
		st.Push(parent.Children[i])
	}

	var obList []OrderBy

	var node *graph.Node
	var ok bool
	var err error

	for {
		if err != nil {
			return fmt.Errorf("argument '%s', %w", node.Name, err)
		}

		if st.Len() == 0 {
			break
		}

		intf := st.Pop()
		node, ok = intf.(*graph.Node)
		if !ok {
			err = fmt.Errorf("unexpected value '%v' (%t)", intf, intf)
			continue
		}

		// Check for type
		if node.Type != graph.NodeStr &&
			node.Type != graph.NodeObj &&
			node.Type != graph.NodeList &&
			node.Type != graph.NodeLabel {
			err = fmt.Errorf("expecting a string, object or list")
			continue
		}

		var ob OrderBy
		ti := sel.Ti
		cn := node

		switch node.Type {
		case graph.NodeStr, graph.NodeLabel:
			if ob.Order, err = toOrder(node.Val); err != nil { // sets the asc desc etc
				continue
			}

		case graph.NodeList:
			if ob, err = orderByFromList(node); err != nil {
				continue
			}

		case graph.NodeObj:
			var path []sdata.TPath
			if path, err = co.FindPath(node.Name, sel.Ti.Name, ""); err != nil {
				continue
			}
			ti = path[0].LT

			cn = node.Children[0]
			if ob.Order, err = toOrder(cn.Val); err != nil { // sets the asc desc etc
				continue
			}

			for i := len(path) - 1; i >= 0; i-- {
				p := path[i]
				rel := sdata.PathToRel(p)
				sel.Joins = append(sel.Joins, Join{
					Rel:    rel,
					Filter: buildFilter(rel, -1),
					Local:  true,
				})
			}
		}

		if err = co.setOrderByColName(ti, &ob, cn); err != nil {
			continue
		}

		if _, ok := cm[ob.Col.Name]; ok {
			err = fmt.Errorf("can only be defined once")
			continue
		}
		cm[ob.Col.Name] = struct{}{}
		obList = append(obList, ob)
	}

	for i := len(obList) - 1; i >= 0; i-- {
		sel.OrderBy = append(sel.OrderBy, obList[i])
	}

	return err
}

func orderByFromList(parent *graph.Node) (ob OrderBy, err error) {
	if len(parent.Children) != 2 {
		return ob, fmt.Errorf(`valid format is [values, order] (eg. [$list, "desc"])`)
	}

	valNode := parent.Children[0]
	orderNode := parent.Children[1]

	ob.Var = valNode.Val

	if ob.Order, err = toOrder(orderNode.Val); err != nil {
		return ob, err
	}
	return ob, nil
}

func (co *Compiler) compileArgOrderByVar(sel *Select, node *graph.Node, cm map[string]struct{}) error {
	for k, v := range sel.tc.OrderBy {
		if err := compileOrderBy(sel, node.Val, k, v, cm); err != nil {
			return err
		}
	}

	return nil
}

func compileOrderBy(sel *Select,
	keyVar, key string,
	values [][2]string,
	cm map[string]struct{}) error {

	obList := make([]OrderBy, 0, len(values))

	for _, v := range values {
		ob := OrderBy{KeyVar: keyVar, Key: key}
		ob.Order, _ = toOrder(v[1])

		col, err := sel.Ti.GetColumn(v[0])
		if err != nil {
			return err
		}
		ob.Col = col
		if _, ok := cm[ob.Col.Name]; ok {
			return fmt.Errorf("duplicate column '%s'", ob.Col.Name)
		}
		obList = append(obList, ob)
	}
	sel.OrderBy = append(sel.OrderBy, obList...)
	return nil
}

func (co *Compiler) compileArgArgs(sel *Select, arg *graph.Arg) error {
	if sel.Ti.Type != "function" {
		return fmt.Errorf("'%s' is not a db function", sel.Ti.Name)
	}

	fn := sel.Ti.Func
	if len(fn.Inputs) == 0 {
		return fmt.Errorf("db function '%s' does not have any arguments", sel.Ti.Name)
	}

	node := arg.Val

	if node.Type != graph.NodeList {
		return argErr("args", "list")
	}

	for i, n := range node.Children {
		var err error
		a := Arg{DType: fn.Inputs[i].Type}

		switch n.Type {
		case graph.NodeLabel:
			a.Type = ArgTypeCol
			a.Col, err = sel.Ti.GetColumn(n.Val)
		case graph.NodeVar:
			a.Type = ArgTypeVar
			fallthrough
		default:
			a.Val = n.Val
		}
		if err != nil {
			return err
		}
		sel.Args = append(sel.Args, a)
	}

	return nil
}

func toOrder(val string) (Order, error) {
	switch val {
	case "asc":
		return OrderAsc, nil
	case "desc":
		return OrderDesc, nil
	case "asc_nulls_first":
		return OrderAscNullsFirst, nil
	case "desc_nulls_first":
		return OrderDescNullsFirst, nil
	case "asc_nulls_last":
		return OrderAscNullsLast, nil
	case "desc_nulls_last":
		return OrderDescNullsLast, nil
	default:
		return OrderAsc, fmt.Errorf("valid values include asc, desc, asc_nulls_first and desc_nulls_first")
	}
}

func (co *Compiler) compileArgDistinctOn(sel *Select, arg *graph.Arg) error {
	node := arg.Val

	if node.Type != graph.NodeList && node.Type != graph.NodeStr {
		return fmt.Errorf("expecting a list of strings or just a string")
	}

	if node.Type == graph.NodeStr {
		if col, err := sel.Ti.GetColumn(node.Val); err == nil {
			switch co.s.DBType() {
			case "mysql":
				sel.OrderBy = append(sel.OrderBy, OrderBy{Order: OrderAsc, Col: col})
			default:
				sel.DistinctOn = append(sel.DistinctOn, col)
			}
		} else {
			return err
		}
	}

	for _, cn := range node.Children {
		if col, err := sel.Ti.GetColumn(cn.Val); err == nil {
			switch co.s.DBType() {
			case "mysql":
				sel.OrderBy = append(sel.OrderBy, OrderBy{Order: OrderAsc, Col: col})
			default:
				sel.DistinctOn = append(sel.DistinctOn, col)
			}
		} else {
			return err
		}
	}

	return nil
}

func (co *Compiler) compileArgLimit(sel *Select, arg *graph.Arg) error {
	node := arg.Val

	if node.Type != graph.NodeNum && node.Type != graph.NodeVar {
		return argTypeErr("number or variable")
	}

	switch node.Type {
	case graph.NodeNum:
		if n, err := strconv.ParseInt(node.Val, 10, 32); err != nil {
			return err
		} else {
			sel.Paging.Limit = int32(n)
		}

	case graph.NodeVar:
		if co.s.DBType() == "mysql" {
			return dbArgErr("limit", "number", "mysql")
		}
		sel.Paging.LimitVar = node.Val
	}
	return nil
}

func (co *Compiler) compileArgOffset(sel *Select, arg *graph.Arg) error {
	node := arg.Val

	if node.Type != graph.NodeNum && node.Type != graph.NodeVar {
		return argTypeErr("number or variable")
	}

	switch node.Type {
	case graph.NodeNum:
		if n, err := strconv.ParseInt(node.Val, 10, 32); err != nil {
			return err
		} else {
			sel.Paging.Offset = int32(n)
		}

	case graph.NodeVar:
		if co.s.DBType() == "mysql" {
			return dbArgErr("limit", "number", "mysql")
		}
		sel.Paging.OffsetVar = node.Val
	}
	return nil
}

func (co *Compiler) compileArgFirstLast(sel *Select, arg *graph.Arg, order Order) error {
	if err := co.compileArgLimit(sel, arg); err != nil {
		return err
	}

	if !sel.Singular {
		sel.Paging.Cursor = true
	}

	sel.order = order
	return nil
}

func (co *Compiler) compileArgAfterBefore(sel *Select, arg *graph.Arg, pt PagingType) error {
	node := arg.Val

	if node.Type != graph.NodeVar || node.Val != "cursor" {
		return fmt.Errorf("value for argument '%s' must be a variable named $cursor", arg.Name)
	}
	sel.Paging.Type = pt
	if !sel.Singular {
		sel.Paging.Cursor = true
	}

	return nil
}

func (co *Compiler) setOrderByColName(ti sdata.DBTable, ob *OrderBy, node *graph.Node) error {
	var name string

	if co.c.EnableCamelcase {
		name = util.ToSnake(node.Name)
	} else {
		name = node.Name
	}

	col, err := ti.GetColumn(name)
	if err != nil {
		return err
	}
	ob.Col = col
	return nil
}

func (co *Compiler) compileAndSetFilter(sel *Select, selID int32, fil *Filter, arg *graph.Arg, role string) error {
	st := util.NewStackInf()
	ex, nu, err := co.compileArgObj(sel.Table, sel.Ti, st, arg, selID)
	if err != nil {
		return err
	}

	if nu && role == "anon" {
		sel.SkipRender = SkipTypeUserNeeded
	}
	setFilter(fil, ex)
	return nil
}

func setFilter(fil *Filter, ex *Exp) {
	if fil.Exp == nil {
		fil.Exp = ex
		return
	}
	// save exiting exp pointer (could be a common one from filter config)
	ow := fil.Exp

	// add a new `and` exp and hook the above saved exp pointer a child
	// we don't want to modify an exp object thats common (from filter config)
	if ow.Op != OpAnd && ow.Op != OpOr && ow.Op != OpNot {
		fil.Exp = newExpOp(OpAnd)
		fil.Exp.Children = fil.Exp.childrenA[:2]
		fil.Exp.Children[0] = ex
		fil.Exp.Children[1] = ow
	} else {
		fil.Exp.Children = append(fil.Exp.Children, ex)
	}
}

func compileFilter(s *sdata.DBSchema, ti sdata.DBTable, filter []string, isJSON bool) (*Exp, bool, error) {
	var fl *Exp
	var needsUser bool

	co := &Compiler{s: s}
	st := util.NewStackInf()

	if len(filter) == 0 {
		return newExp(), false, nil
	}

	for _, v := range filter {
		if v == "false" {
			return newExpOp(OpFalse), false, nil
		}

		node, err := graph.ParseArgValue(v, isJSON)
		if err != nil {
			return nil, false, err
		}

		f, nu, err := co.compileArgNode("", ti, st, node, isJSON, -1)
		if err != nil {
			return nil, false, err
		}

		if nu {
			needsUser = true
		}

		// TODO: Invalid table names in nested where causes fail silently
		// returning a nil 'f' this needs to be fixed

		// TODO: Invalid where clauses such as missing op (eg. eq) also fail silently

		if fl == nil {
			if len(filter) == 1 {
				fl = f
				continue
			} else {
				fl = newExpOp(OpAnd)
			}
		}
		fl.Children = append(fl.Children, f)
	}

	return fl, needsUser, nil
}

// func buildPath(a []string) string {
// 	switch len(a) {
// 	case 0:
// 		return ""
// 	case 1:
// 		return a[0]
// 	}

// 	n := len(a) - 1
// 	for i := 0; i < len(a); i++ {
// 		n += len(a[i])
// 	}

// 	var b strings.Builder
// 	b.Grow(n)
// 	b.WriteString(a[0])
// 	for _, s := range a[1:] {
// 		b.WriteRune('.')
// 		b.WriteString(s)
// 	}
// 	return b.String()
// }

func ifArgList(arg graph.Arg, lty graph.ParserType) bool {
	return arg.Val.Type == graph.NodeList &&
		len(arg.Val.Children) != 0 &&
		arg.Val.Children[0].Type == lty
}

func ifArg(arg graph.Arg, ty graph.ParserType) bool {
	return arg.Val.Type == ty
}

func ifNotArg(arg graph.Arg, ty graph.ParserType) bool {
	return arg.Val.Type != ty
}

// func ifArgVal(arg graph.Arg, val string) bool {
// 	return arg.Val.Val == val
// }

func ifNotArgVal(arg graph.Arg, val string) bool {
	return arg.Val.Val != val
}

func argErr(name, ty string) error {
	return fmt.Errorf("value for argument '%s' must be a %s", name, ty)
}

func argTypeErr(ty string) error {
	return fmt.Errorf("value must be a %s", ty)
}

func dbArgErr(name, ty, db string) error {
	return fmt.Errorf("%s: value for argument '%s' must be a %s", db, name, ty)
}

func (sel *Select) addIArg(arg Arg) {
	sel.IArgs = append(sel.IArgs, arg)
}

func (sel *Select) GetInternalArg(name string) (Arg, bool) {
	var arg Arg
	for _, v := range sel.IArgs {
		if v.Name == name {
			return v, true
		}
	}
	return arg, false
}

func (s *Script) HasReqFn() bool {
	return s.SC.HasRequestFn()
}

func (s *Script) HasRespFn() bool {
	return s.SC.HasResponseFn()
}

/*
func (qc *QCode) getVar(name string, vt ValType) (string, error) {
	val, ok := qc.Vars[name]
	if !ok {
		return "", fmt.Errorf("variable '%s' not defined", name)
	}
	k := string(val)
	if k == "null" {
		return "", nil
	}
	switch vt {
	case ValStr:
		if k != "" && k[0] == '"' {
			return k[1:(len(k) - 1)], nil
		}
	case ValNum:
		if k != "" && ((k[0] >= '0' && k[0] <= '9') || k[0] == '-') {
			return k, nil
		}
	case ValBool:
		if strings.EqualFold(k, "true") || strings.EqualFold(k, "false") {
			return k, nil
		}
	case ValList:
		if k != "" && k[0] == '[' {
			return k, nil
		}
	case ValObj:
		if k != "" && k[0] == '{' {
			return k, nil
		}
	}

	var vts string
	switch vt {
	case ValStr:
		vts = "string"
	case ValNum:
		vts = "number"
	case ValBool:
		vts = "boolean"
	case ValList:
		vts = "list"
	case ValObj:
		vts = "object"
	}

	return "", fmt.Errorf("variable '%s' must be a %s and not '%s'",
		name, vts, k)
}
*/
