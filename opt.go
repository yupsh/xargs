package command

type MaxArgs int
type MaxLines int
type MaxChars int
type MaxProcs int
type Delimiter string
type ReplaceStr string

type NullDelimFlag bool

const (
	NullDelim   NullDelimFlag = true
	NoNullDelim NullDelimFlag = false
)

type PrintFlag bool

const (
	Print   PrintFlag = true
	NoPrint PrintFlag = false
)

type InteractiveFlag bool

const (
	Interactive   InteractiveFlag = true
	NoInteractive InteractiveFlag = false
)

type NoRunEmptyFlag bool

const (
	NoRunEmpty NoRunEmptyFlag = true
	RunEmpty   NoRunEmptyFlag = false
)

type VerboseFlag bool

const (
	Verbose   VerboseFlag = true
	NoVerbose VerboseFlag = false
)

type flags struct {
	MaxArgs     MaxArgs
	MaxLines    MaxLines
	MaxChars    MaxChars
	MaxProcs    MaxProcs
	Delimiter   Delimiter
	ReplaceStr  ReplaceStr
	NullDelim   NullDelimFlag
	Print       PrintFlag
	Interactive InteractiveFlag
	NoRunEmpty  NoRunEmptyFlag
	Verbose     VerboseFlag
}

func (m MaxArgs) Configure(flags *flags)         { flags.MaxArgs = m }
func (m MaxLines) Configure(flags *flags)        { flags.MaxLines = m }
func (m MaxChars) Configure(flags *flags)        { flags.MaxChars = m }
func (m MaxProcs) Configure(flags *flags)        { flags.MaxProcs = m }
func (d Delimiter) Configure(flags *flags)       { flags.Delimiter = d }
func (r ReplaceStr) Configure(flags *flags)      { flags.ReplaceStr = r }
func (f NullDelimFlag) Configure(flags *flags)   { flags.NullDelim = f }
func (f PrintFlag) Configure(flags *flags)       { flags.Print = f }
func (f InteractiveFlag) Configure(flags *flags) { flags.Interactive = f }
func (f NoRunEmptyFlag) Configure(flags *flags)  { flags.NoRunEmpty = f }
func (f VerboseFlag) Configure(flags *flags)     { flags.Verbose = f }
