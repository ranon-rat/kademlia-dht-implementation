package executioner

var executableEnv = []string{}

func Setup() {
	executableEnv = LoadLocalEnviroments()

}
