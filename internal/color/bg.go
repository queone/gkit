package color

// Background-color helper family — 121 helpers (10 hues × 11 steps + 11 heat
// steps) that mirror the foreground ramps in color.go. Each Bg<Hue><Step>
// emits the same 256-color index as its foreground twin (e.g. BgGra5 uses
// index 245, matching Gra5), but with SGR prefix 48;5; (background) instead
// of 38;5; (foreground).
//
// Composable with Bold, Reverse, and the foreground helpers via the same
// resetCodeRe rewrite trick: BgGra2(Whi9("CAREER")) renders light-gray text
// on a dim-gray background; Bold(BgGra2(Whi9("x"))) further makes it bold.
// Same gating as the foreground helpers — when enabled is false or color256
// is false, the input is returned unwrapped.

// BgGra — gray background ramp.
func BgGra0(v any) string  { return bgWrap("48;5;16", v) }
func BgGra1(v any) string  { return bgWrap("48;5;233", v) }
func BgGra2(v any) string  { return bgWrap("48;5;236", v) }
func BgGra3(v any) string  { return bgWrap("48;5;239", v) }
func BgGra4(v any) string  { return bgWrap("48;5;242", v) }
func BgGra5(v any) string  { return bgWrap("48;5;245", v) }
func BgGra6(v any) string  { return bgWrap("48;5;248", v) }
func BgGra7(v any) string  { return bgWrap("48;5;251", v) }
func BgGra8(v any) string  { return bgWrap("48;5;254", v) }
func BgGra9(v any) string  { return bgWrap("48;5;255", v) }
func BgGra10(v any) string { return bgWrap("48;5;231", v) }

// BgRed — red background ramp.
func BgRed0(v any) string  { return bgWrap("48;5;16", v) }
func BgRed1(v any) string  { return bgWrap("48;5;52", v) }
func BgRed2(v any) string  { return bgWrap("48;5;88", v) }
func BgRed3(v any) string  { return bgWrap("48;5;124", v) }
func BgRed4(v any) string  { return bgWrap("48;5;160", v) }
func BgRed5(v any) string  { return bgWrap("48;5;196", v) }
func BgRed6(v any) string  { return bgWrap("48;5;203", v) }
func BgRed7(v any) string  { return bgWrap("48;5;210", v) }
func BgRed8(v any) string  { return bgWrap("48;5;217", v) }
func BgRed9(v any) string  { return bgWrap("48;5;224", v) }
func BgRed10(v any) string { return bgWrap("48;5;231", v) }

// BgOrg — orange background ramp.
func BgOrg0(v any) string  { return bgWrap("48;5;16", v) }
func BgOrg1(v any) string  { return bgWrap("48;5;88", v) }
func BgOrg2(v any) string  { return bgWrap("48;5;130", v) }
func BgOrg3(v any) string  { return bgWrap("48;5;166", v) }
func BgOrg4(v any) string  { return bgWrap("48;5;172", v) }
func BgOrg5(v any) string  { return bgWrap("48;5;208", v) }
func BgOrg6(v any) string  { return bgWrap("48;5;214", v) }
func BgOrg7(v any) string  { return bgWrap("48;5;215", v) }
func BgOrg8(v any) string  { return bgWrap("48;5;222", v) }
func BgOrg9(v any) string  { return bgWrap("48;5;229", v) }
func BgOrg10(v any) string { return bgWrap("48;5;230", v) }

// BgYel — yellow background ramp.
func BgYel0(v any) string  { return bgWrap("48;5;16", v) }
func BgYel1(v any) string  { return bgWrap("48;5;58", v) }
func BgYel2(v any) string  { return bgWrap("48;5;100", v) }
func BgYel3(v any) string  { return bgWrap("48;5;142", v) }
func BgYel4(v any) string  { return bgWrap("48;5;184", v) }
func BgYel5(v any) string  { return bgWrap("48;5;220", v) }
func BgYel6(v any) string  { return bgWrap("48;5;226", v) }
func BgYel7(v any) string  { return bgWrap("48;5;227", v) }
func BgYel8(v any) string  { return bgWrap("48;5;228", v) }
func BgYel9(v any) string  { return bgWrap("48;5;229", v) }
func BgYel10(v any) string { return bgWrap("48;5;230", v) }

// BgGrn — green background ramp.
func BgGrn0(v any) string  { return bgWrap("48;5;16", v) }
func BgGrn1(v any) string  { return bgWrap("48;5;22", v) }
func BgGrn2(v any) string  { return bgWrap("48;5;28", v) }
func BgGrn3(v any) string  { return bgWrap("48;5;34", v) }
func BgGrn4(v any) string  { return bgWrap("48;5;40", v) }
func BgGrn5(v any) string  { return bgWrap("48;5;46", v) }
func BgGrn6(v any) string  { return bgWrap("48;5;83", v) }
func BgGrn7(v any) string  { return bgWrap("48;5;120", v) }
func BgGrn8(v any) string  { return bgWrap("48;5;157", v) }
func BgGrn9(v any) string  { return bgWrap("48;5;194", v) }
func BgGrn10(v any) string { return bgWrap("48;5;231", v) }

// BgCya — cyan background ramp.
func BgCya0(v any) string  { return bgWrap("48;5;16", v) }
func BgCya1(v any) string  { return bgWrap("48;5;23", v) }
func BgCya2(v any) string  { return bgWrap("48;5;30", v) }
func BgCya3(v any) string  { return bgWrap("48;5;37", v) }
func BgCya4(v any) string  { return bgWrap("48;5;44", v) }
func BgCya5(v any) string  { return bgWrap("48;5;51", v) }
func BgCya6(v any) string  { return bgWrap("48;5;87", v) }
func BgCya7(v any) string  { return bgWrap("48;5;123", v) }
func BgCya8(v any) string  { return bgWrap("48;5;159", v) }
func BgCya9(v any) string  { return bgWrap("48;5;195", v) }
func BgCya10(v any) string { return bgWrap("48;5;231", v) }

// BgBlu — blue background ramp.
func BgBlu0(v any) string  { return bgWrap("48;5;16", v) }
func BgBlu1(v any) string  { return bgWrap("48;5;17", v) }
func BgBlu2(v any) string  { return bgWrap("48;5;18", v) }
func BgBlu3(v any) string  { return bgWrap("48;5;19", v) }
func BgBlu4(v any) string  { return bgWrap("48;5;20", v) }
func BgBlu5(v any) string  { return bgWrap("48;5;21", v) }
func BgBlu6(v any) string  { return bgWrap("48;5;27", v) }
func BgBlu7(v any) string  { return bgWrap("48;5;33", v) }
func BgBlu8(v any) string  { return bgWrap("48;5;75", v) }
func BgBlu9(v any) string  { return bgWrap("48;5;111", v) }
func BgBlu10(v any) string { return bgWrap("48;5;195", v) }

// BgPur — purple background ramp.
func BgPur0(v any) string  { return bgWrap("48;5;16", v) }
func BgPur1(v any) string  { return bgWrap("48;5;53", v) }
func BgPur2(v any) string  { return bgWrap("48;5;54", v) }
func BgPur3(v any) string  { return bgWrap("48;5;55", v) }
func BgPur4(v any) string  { return bgWrap("48;5;91", v) }
func BgPur5(v any) string  { return bgWrap("48;5;92", v) }
func BgPur6(v any) string  { return bgWrap("48;5;99", v) }
func BgPur7(v any) string  { return bgWrap("48;5;105", v) }
func BgPur8(v any) string  { return bgWrap("48;5;141", v) }
func BgPur9(v any) string  { return bgWrap("48;5;177", v) }
func BgPur10(v any) string { return bgWrap("48;5;219", v) }

// BgMag — magenta background ramp.
func BgMag0(v any) string  { return bgWrap("48;5;16", v) }
func BgMag1(v any) string  { return bgWrap("48;5;53", v) }
func BgMag2(v any) string  { return bgWrap("48;5;90", v) }
func BgMag3(v any) string  { return bgWrap("48;5;127", v) }
func BgMag4(v any) string  { return bgWrap("48;5;164", v) }
func BgMag5(v any) string  { return bgWrap("48;5;201", v) }
func BgMag6(v any) string  { return bgWrap("48;5;207", v) }
func BgMag7(v any) string  { return bgWrap("48;5;213", v) }
func BgMag8(v any) string  { return bgWrap("48;5;219", v) }
func BgMag9(v any) string  { return bgWrap("48;5;225", v) }
func BgMag10(v any) string { return bgWrap("48;5;231", v) }

// BgWhi — white background ramp.
func BgWhi0(v any) string  { return bgWrap("48;5;16", v) }
func BgWhi1(v any) string  { return bgWrap("48;5;59", v) }
func BgWhi2(v any) string  { return bgWrap("48;5;102", v) }
func BgWhi3(v any) string  { return bgWrap("48;5;145", v) }
func BgWhi4(v any) string  { return bgWrap("48;5;188", v) }
func BgWhi5(v any) string  { return bgWrap("48;5;231", v) }
func BgWhi6(v any) string  { return bgWrap("48;5;252", v) }
func BgWhi7(v any) string  { return bgWrap("48;5;253", v) }
func BgWhi8(v any) string  { return bgWrap("48;5;254", v) }
func BgWhi9(v any) string  { return bgWrap("48;5;255", v) }
func BgWhi10(v any) string { return bgWrap("48;5;15", v) }

// BgHeat — heat background ramp (red → yellow → green).
func BgHeat0(v any) string  { return bgWrap("48;5;196", v) }
func BgHeat1(v any) string  { return bgWrap("48;5;202", v) }
func BgHeat2(v any) string  { return bgWrap("48;5;208", v) }
func BgHeat3(v any) string  { return bgWrap("48;5;214", v) }
func BgHeat4(v any) string  { return bgWrap("48;5;220", v) }
func BgHeat5(v any) string  { return bgWrap("48;5;226", v) }
func BgHeat6(v any) string  { return bgWrap("48;5;190", v) }
func BgHeat7(v any) string  { return bgWrap("48;5;154", v) }
func BgHeat8(v any) string  { return bgWrap("48;5;118", v) }
func BgHeat9(v any) string  { return bgWrap("48;5;82", v) }
func BgHeat10(v any) string { return bgWrap("48;5;46", v) }
