package opt

// Custom types for parameters
type MaxArgs int
type MaxLines int
type MaxChars int
type MaxProcs int
type Delimiter string
type ReplaceStr string

// Boolean flag types with constants
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
	NoRunEmpty   NoRunEmptyFlag = true
	RunEmpty     NoRunEmptyFlag = false
)

type VerboseFlag bool
const (
	Verbose   VerboseFlag = true
	NoVerbose VerboseFlag = false
)

// Flags represents the configuration options for the xargs command
type Flags struct {
	MaxArgs     MaxArgs         // Maximum number of arguments per command line (-n)
	MaxLines    MaxLines        // Maximum number of input lines per command line (-L)
	MaxChars    MaxChars        // Maximum number of characters per command line (-s)
	MaxProcs    MaxProcs        // Maximum number of processes to run in parallel (-P)
	Delimiter   Delimiter       // Input delimiter (-d)
	ReplaceStr  ReplaceStr      // Replace string in initial arguments (-I)
	NullDelim   NullDelimFlag   // Use null character as delimiter (-0)
	Print       PrintFlag       // Print each command line before executing (-t)
	Interactive InteractiveFlag // Prompt before executing each command (-p)
	NoRunEmpty  NoRunEmptyFlag  // Don't run if no arguments (-r)
	Verbose     VerboseFlag     // Verbose output (-v)
}

// Configure methods for the opt system
func (m MaxArgs) Configure(flags *Flags)         { flags.MaxArgs = m }
func (m MaxLines) Configure(flags *Flags)        { flags.MaxLines = m }
func (m MaxChars) Configure(flags *Flags)        { flags.MaxChars = m }
func (m MaxProcs) Configure(flags *Flags)        { flags.MaxProcs = m }
func (d Delimiter) Configure(flags *Flags)       { flags.Delimiter = d }
func (r ReplaceStr) Configure(flags *Flags)      { flags.ReplaceStr = r }
func (f NullDelimFlag) Configure(flags *Flags)   { flags.NullDelim = f }
func (f PrintFlag) Configure(flags *Flags)       { flags.Print = f }
func (f InteractiveFlag) Configure(flags *Flags) { flags.Interactive = f }
func (f NoRunEmptyFlag) Configure(flags *Flags)  { flags.NoRunEmpty = f }
func (f VerboseFlag) Configure(flags *Flags)     { flags.Verbose = f }
