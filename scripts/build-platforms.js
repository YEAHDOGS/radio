import { spawnSync } from 'node:child_process';

const COMMANDS = { android: ['tauri', 'android', ['build']], ios: ['tauri', 'ios', ['build']], macos: ['tauri', 'build', []], windows: ['tauri', 'build', []], linux: ['tauri', 'build', []] };
const selected = process.argv.slice(2).filter((platform) => Object.hasOwn(COMMANDS, platform));
for (const platform of selected) {
  const [command, subcommand, args] = COMMANDS[platform];
  const result = spawnSync('npx', [command, subcommand, ...args], { stdio: 'inherit', shell: process.platform === 'win32' });
  if (result.status !== 0) process.exit(result.status ?? 1);
}
