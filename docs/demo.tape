Output assets/demo.gif

Set Shell "zsh"
Set FontFamily "SFMono Nerd Font"
Set FontSize 15
Set Width 1000
Set Height 520
Set Padding 20
Set Theme "Dracula"

Sleep 200ms

Type "# build hwcfgmap from source"
Enter
Sleep 300ms
Type "go build -o ./hwcfgmap ./cmd/hwcfgmap"
Enter
Sleep 1500ms

Type "# probe the box — GPU / CPU / RAM / NVMe → BoxProfile JSON"
Enter
Sleep 300ms
Type "./hwcfgmap probe"
Enter
Sleep 1200ms

Type "# probe + synthesize a model target → ArgMatrix + pasteable llama-server line"
Enter
Sleep 300ms
Type "./hwcfgmap probe --model qwen3-27b"
Enter
Sleep 1500ms

Type "# machine-readable: just the launch line for scripts"
Enter
Sleep 200ms
Type "./hwcfgmap probe --model qwen3-27b --launch-only"
Enter
Sleep 800ms

Type "# list registered model targets"
Enter
Sleep 200ms
Type "./hwcfgmap models"
Enter
Sleep 800ms

Sleep 800ms
