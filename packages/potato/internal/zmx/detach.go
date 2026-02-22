package zmx

import "os/exec"

func DetachAll() {
	exec.Command("zmx", "detach-all").Run()
}
