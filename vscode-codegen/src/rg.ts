import * as os from 'os'
import * as fs from 'fs'
import { exec } from 'child_process'
import path from 'path'

export async function getRgPath(extensionPath: string): Promise<string | null> {
	const target = await getTarget()
	const resourcesDir = path.join(extensionPath, 'resources', 'bin')
	const files = await new Promise<string[]>((resolve, reject) => {
		fs.readdir(resourcesDir, (err, files) => {
			if (err) {
				reject(err)
				return
			}
			resolve(files)
		})
	})
	for (const file of files) {
		if (file.indexOf(target) !== -1) {
			return path.join(resourcesDir, file)
		}
	}
	return null
}

// Code below this line copied from https://github.com/microsoft/vscode-ripgrep

async function isMusl() {
	let stderr
	try {
		stderr = (await exec('ldd --version')).stderr
	} catch (err) {
		stderr = (err as any).stderr
	}
	if (stderr.indexOf('musl') > -1) {
		return true
	}
	return false
}

async function getTarget(): Promise<string> {
	const arch = process.env.npm_config_arch || os.arch()

	switch (os.platform()) {
		case 'darwin':
			return arch === 'arm64' ? 'aarch64-apple-darwin' : 'x86_64-apple-darwin'
		case 'win32':
			return arch === 'x64'
				? 'x86_64-pc-windows-msvc'
				: arch === 'arm'
				? 'aarch64-pc-windows-msvc'
				: 'i686-pc-windows-msvc'
		case 'linux':
			return arch === 'x64'
				? 'x86_64-unknown-linux-musl'
				: arch === 'arm'
				? 'arm-unknown-linux-gnueabihf'
				: arch === 'armv7l'
				? 'arm-unknown-linux-gnueabihf'
				: arch === 'arm64'
				? (await isMusl())
					? 'aarch64-unknown-linux-musl'
					: 'aarch64-unknown-linux-gnu'
				: arch === 'ppc64'
				? 'powerpc64le-unknown-linux-gnu'
				: arch === 's390x'
				? 's390x-unknown-linux-gnu'
				: 'i686-unknown-linux-musl'
		default:
			throw new Error('Unknown platform: ' + os.platform())
	}
}
