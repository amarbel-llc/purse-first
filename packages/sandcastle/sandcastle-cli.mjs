#!/usr/bin/env node
import { Command } from 'commander'
import { SandboxManager } from './index.js'
import { spawn } from 'child_process'
import { logForDebugging } from './utils/debug.js'
import { loadConfig, loadConfigFromString } from './utils/config-loader.js'
import * as readline from 'readline'
import * as fs from 'fs'
import * as path from 'path'
import * as os from 'os'

function getDefaultConfigPath() {
  return path.join(os.homedir(), '.srt-settings.json')
}

function getDefaultConfig() {
  return {
    network: {
      allowedDomains: [],
      deniedDomains: [],
    },
    filesystem: {
      denyRead: [],
      allowWrite: [],
      denyWrite: [],
    },
  }
}

function shellQuote(s) {
  return "'" + s.replace(/'/g, "'\\''") + "'"
}

async function main() {
  const program = new Command()

  program
    .name('sandcastle')
    .description(
      'Run commands in a sandbox with network and filesystem restrictions',
    )
    .argument('[command...]', 'command to run in the sandbox')
    .option('-d, --debug', 'enable debug logging')
    .option(
      '--config <path>',
      'path to config file (default: ~/.srt-settings.json)',
    )
    .option('--shell <shell>', 'shell to execute the command with')
    .option('--tmpdir <path>', 'override the temporary directory used inside the sandbox')
    .option(
      '--control-fd <fd>',
      'read config updates from file descriptor (JSON lines protocol)',
      parseInt,
    )
    .allowUnknownOption()
    .action(async (commandArgs, options) => {
      try {
        if (options.debug) {
          process.env.DEBUG = 'true'
        }

        const configPath = options.config || getDefaultConfigPath()
        let runtimeConfig = loadConfig(configPath)

        if (!runtimeConfig) {
          logForDebugging(
            `No config found at ${configPath}, using default config`,
          )
          runtimeConfig = getDefaultConfig()
        }

        let sandboxTmpdir
        let cleanupTmpdir = false

        if (options.tmpdir) {
          sandboxTmpdir = options.tmpdir
          fs.mkdirSync(sandboxTmpdir, { recursive: true })
        } else {
          sandboxTmpdir = fs.mkdtempSync(path.join(os.tmpdir(), 'sandcastle-'))
          cleanupTmpdir = true
        }

        SandboxManager.setTmpdir(sandboxTmpdir)

        process.on('exit', () => {
          if (cleanupTmpdir && sandboxTmpdir) {
            try {
              fs.rmSync(sandboxTmpdir, { recursive: true, force: true })
            } catch {
              // Best-effort cleanup
            }
          }
        })

        logForDebugging('Initializing sandbox...')
        await SandboxManager.initialize(runtimeConfig)

        let controlReader = null
        if (options.controlFd !== undefined) {
          try {
            const controlStream = fs.createReadStream('', {
              fd: options.controlFd,
            })
            controlReader = readline.createInterface({
              input: controlStream,
              crlfDelay: Infinity,
            })

            controlReader.on('line', (line) => {
              const newConfig = loadConfigFromString(line)
              if (newConfig) {
                logForDebugging(
                  `Config updated from control fd: ${JSON.stringify(newConfig)}`,
                )
                SandboxManager.updateConfig(newConfig)
              } else if (line.trim()) {
                logForDebugging(
                  `Invalid config on control fd (ignored): ${line}`,
                )
              }
            })

            controlReader.on('error', (err) => {
              logForDebugging(`Control fd error: ${err.message}`)
            })

            logForDebugging(
              `Listening for config updates on fd ${options.controlFd}`,
            )
          } catch (err) {
            logForDebugging(
              `Failed to open control fd ${options.controlFd}: ${err instanceof Error ? err.message : String(err)}`,
            )
          }
        }

        process.on('exit', () => {
          controlReader?.close()
        })

        let command
        if (commandArgs.length > 0) {
          if (options.shell) {
            const quoted = commandArgs.map(shellQuote).join(' ')
            command = `${options.shell} -c ${shellQuote(quoted)}`
          } else {
            command = commandArgs.map(shellQuote).join(' ')
          }
          logForDebugging(`Command: ${command}`)
        } else {
          console.error(
            'Error: No command specified. Provide command arguments.',
          )
          process.exit(1)
        }

        logForDebugging(
          JSON.stringify(
            SandboxManager.getNetworkRestrictionConfig(),
            null,
            2,
          ),
        )

        const sandboxedCommand = await SandboxManager.wrapWithSandbox(command)

        const child = spawn(sandboxedCommand, {
          shell: true,
          stdio: 'inherit',
        })

        child.on('exit', (code, signal) => {
          SandboxManager.cleanupAfterCommand()

          if (signal) {
            if (signal === 'SIGINT' || signal === 'SIGTERM') {
              process.exit(0)
            } else {
              console.error(`Process killed by signal: ${signal}`)
              process.exit(1)
            }
          }
          process.exit(code ?? 0)
        })

        child.on('error', (error) => {
          console.error(`Failed to execute command: ${error.message}`)
          process.exit(1)
        })

        process.on('SIGINT', () => {
          child.kill('SIGINT')
        })

        process.on('SIGTERM', () => {
          child.kill('SIGTERM')
        })
      } catch (error) {
        console.error(
          `Error: ${error instanceof Error ? error.message : String(error)}`,
        )
        process.exit(1)
      }
    })

  program.parse()
}

main().catch((error) => {
  console.error('Fatal error:', error)
  process.exit(1)
})
