param([Parameter(Mandatory=$true)][string]$Path)

# Parse an ELF file and report: class, machine, type, PT_INTERP, DT_NEEDED, GLIBC_ symbol refs.
$bytes = [IO.File]::ReadAllBytes((Resolve-Path $Path))

if ($bytes.Length -lt 64 -or $bytes[0] -ne 0x7F -or $bytes[1] -ne 0x45 -or $bytes[2] -ne 0x4C -or $bytes[3] -ne 0x46) {
    throw "not an ELF file: $Path"
}

$is64 = $bytes[4] -eq 2
$isLE = $bytes[5] -eq 1

function U16([int]$o) { [BitConverter]::ToUInt16($bytes, $o) }
function U32([int]$o) { [BitConverter]::ToUInt32($bytes, $o) }
function U64([int]$o) { [BitConverter]::ToUInt64($bytes, $o) }

$eType = U16 16
$ePhOff = 0
$ePEntSz = 0
$ePhNum = 0
if ($is64) { $ePhOff = U64 32; $ePEntSz = U16 54; $ePhNum = U16 56 }
else       { $ePhOff = U32 28; $ePEntSz = U16 42; $ePhNum = U16 44 }

$machRaw = U16 18
$machine = switch ($machRaw) {
    0x03 {'i386'} 0x28 {'ARM'} 0x3E {'x86_64'} 0xB7 {'aarch64'}
    0x08 {'MIPS'} 0xF3 {'RISC-V'} 0x102 {'LoongArch'} default { "0x{0:X}" -f $machRaw }
}
$typeName = switch ($eType) { 1 {'ET_REL'} 2 {'ET_EXEC'} 3 {'ET_DYN'} 4 {'ET_CORE'} default {"type=$eType"} }

# collect load segments first (vaddr -> file offset mapping)
$loads = @()
for ($i = 0; $i -lt $ePhNum; $i++) {
    $o = [int]$ePhOff + ($i * $ePEntSz)
    if ((U32 $o) -eq 1) {
        if ($is64) { $loads += ,@((U64 ($o + 16)), (U64 ($o + 8)), (U64 ($o + 32))) }
        else       { $loads += ,@((U32 ($o + 8)),  (U32 ($o + 4)), (U32 ($o + 16))) }
    }
}

$interp = $null
$needed = New-Object System.Collections.Generic.List[string]
$dynamic = $false
$dynEntries = @()

for ($i = 0; $i -lt $ePhNum; $i++) {
    $o = [int]$ePhOff + ($i * $ePEntSz)
    $pType = U32 $o
    if ($is64) { $pOff = U64 ($o + 8); $pFilesz = U64 ($o + 32) }
    else       { $pOff = U32 ($o + 4); $pFilesz = U32 ($o + 16) }

    if ($pType -eq 3) {
        $interp = [Text.Encoding]::ASCII.GetString($bytes, [int]$pOff, [int]$pFilesz - 1)
    }
    if ($pType -eq 2) {
        $dynamic = $true
        $entSz = 16
        if (-not $is64) { $entSz = 8 }
        $cnt = [int]($pFilesz / $entSz)
        for ($k = 0; $k -lt $cnt; $k++) {
            $d = [int]$pOff + $k * $entSz
            if ($is64) { $tag = U64 $d; $val = U64 ($d + 8) } else { $tag = U32 $d; $val = U32 ($d + 4) }
            if ($tag -eq 0) { break }
            $dynEntries += ,@($tag, $val)
        }
    }
}

$strTabVaddr = $null
foreach ($e in $dynEntries) { if ($e[0] -eq 5) { $strTabVaddr = $e[1] } }

if ($strTabVaddr -ne $null) {
    $strFileOff = $null
    foreach ($l in $loads) {
        if ($strTabVaddr -ge $l[0] -and $strTabVaddr -lt ($l[0] + $l[2])) { $strFileOff = [int]($l[1] + ($strTabVaddr - $l[0])); break }
    }
    if ($strFileOff -ne $null) {
        foreach ($e in $dynEntries) {
            if ($e[0] -eq 1) {
                $off = $strFileOff + [int]$e[1]
                $end = $off
                while ($end -lt $bytes.Length -and $bytes[$end] -ne 0) { $end++ }
                $needed.Add([Text.Encoding]::ASCII.GetString($bytes, $off, $end - $off))
            }
        }
    }
}

$ascii = [Text.Encoding]::GetEncoding(28591).GetString($bytes)
$glibc = [regex]::Matches($ascii, 'GLIBC_[0-9]+\.[0-9]+(\.[0-9]+)?') | ForEach-Object { $_.Value } | Sort-Object -Unique

Write-Host ("file      : {0}" -f (Split-Path $Path -Leaf))
Write-Host ("size      : {0:N0} bytes ({1:N1} MB)" -f $bytes.Length, ($bytes.Length / 1MB))
Write-Host ("elf       : {0}-bit {1} {2}" -f $(if($is64){64}else{32}), $machine, $typeName)
if ($interp) { Write-Host ("PT_INTERP : {0}" -f $interp) } else { Write-Host "PT_INTERP : <none>" }
Write-Host ("PT_DYNAMIC: {0}" -f $dynamic)
if ($needed.Count) { Write-Host ("DT_NEEDED : {0}" -f ($needed -join ', ')) } else { Write-Host "DT_NEEDED : <none>" }
if ($glibc) { Write-Host ("GLIBC_*   : {0}" -f ($glibc -join ', ')) } else { Write-Host "GLIBC_*   : <none>" }

$static = (-not $interp) -and ($needed.Count -eq 0) -and (-not $glibc)
if ($static) {
    Write-Host "verdict   : FULLY STATIC - no dynamic loader, no shared libs, no glibc" -ForegroundColor Green
    Write-Host ""
    exit 0
} else {
    Write-Host "verdict   : DYNAMIC - depends on host libraries" -ForegroundColor Red
    Write-Host ""
    exit 1
}
