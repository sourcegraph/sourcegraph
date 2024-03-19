import type { ComponentType } from 'react'

// TODO(id: md-icons-and-react-icons)
//
// We're using react-icons for the React version of the web app
// since it has a large number of icons. However, those aren't
// usable in SvelteKit as the icons are React components.
//
// So we also use Material Design icons which are exposed as SVGs.
// However, this has two drawbacks:
// - It has many fewer languages compared to react-icons.
// - A future version of Material Design icons will remove programming
//   language and file type icons.
//   https://github.com/Templarian/MaterialDesign/issues/6602
//
// It would be valuable to explore other icon libraries that can
// be used both by React and SvelteKit or make our own.
import {
    mdiCodeJson,
    mdiCog,
    mdiConsole,
    mdiDocker,
    mdiEarth,
    mdiFileCodeOutline,
    mdiFileGifBox,
    mdiFileJpgBox,
    mdiFilePngBox,
    mdiGit,
    mdiGraphql,
    mdiLanguageC,
    mdiLanguageCpp,
    mdiLanguageCsharp,
    mdiLanguageCss3,
    mdiLanguageGo,
    mdiLanguageHaskell,
    mdiLanguageHtml5,
    mdiLanguageJava,
    mdiLanguageJavascript,
    mdiLanguageKotlin,
    mdiLanguageLua,
    mdiLanguageMarkdown,
    mdiLanguagePhp,
    mdiLanguagePython,
    mdiLanguageR,
    mdiLanguageRuby,
    mdiLanguageRust,
    mdiLanguageSwift,
    mdiLanguageTypescript,
    mdiNix,
    mdiNpm,
    mdiSass,
    mdiSvg,
    mdiText,
} from '@mdi/js'
import { CiSettings, CiTextAlignLeft } from 'react-icons/ci'
import { FaCss3Alt, FaSass, FaVuejs } from 'react-icons/fa'
import { GoDatabase, GoTerminal } from 'react-icons/go'
import { GrJava } from 'react-icons/gr'
import { ImEarth } from 'react-icons/im'
import { MdGif } from 'react-icons/md'
import { PiFilePngLight } from 'react-icons/pi'
import {
    SiApachegroovy,
    SiC,
    SiClojure,
    SiCmake,
    SiCoffeescript,
    SiCplusplus,
    SiCrystal,
    SiCsharp,
    SiD,
    SiDart,
    SiDocker,
    SiEditorconfig,
    SiElixir,
    SiElm,
    SiErlang,
    SiFortran,
    SiFsharp,
    SiGit,
    SiGnuemacs,
    SiGo,
    SiGraphql,
    SiHaskell,
    SiHtml5,
    SiJavascript,
    SiJinja,
    SiJpeg,
    SiJulia,
    SiKotlin,
    SiLlvm,
    SiLua,
    SiMarkdown,
    SiNginx,
    SiNim,
    SiNixos,
    SiNpm,
    SiOcaml,
    SiPerl,
    SiPhp,
    SiPurescript,
    SiPython,
    SiR,
    SiRuby,
    SiRust,
    SiScala,
    SiSvelte,
    SiSvg,
    SiSwift,
    SiTerraform,
    SiToml,
    SiTypescript,
    SiUnrealengine,
    SiVim,
    SiVisualbasic,
    SiWebassembly,
    SiWolframmathematica,
    SiZig,
} from 'react-icons/si'
import { VscJson } from 'react-icons/vsc'

import styles from './LanguageIcon.module.scss'

export type CustomIcon = ComponentType<{ className?: string }>

export interface ReactIcon {
    icon: CustomIcon
    className: string
}

export interface SvgIcon {
    path: string
    color: string
}

interface UnifiedIcon {
    react: ReactIcon
    svg?: SvgIcon['path']
}

/**
 * The keys of this map must be present in the list of `languageFilter.ALL_LANGUAGES`.
 *
 * This map is deliberately not public, use {@link getFileIconInfo} instead.
 *
 * See FIXME(id: language-detection) for context on why this map uses the
 * language as the key instead of something simpler like a file extension.
 */
export const FILE_ICONS_BY_LANGUAGE: Map<string, UnifiedIcon> = new Map([
    ['Bash', { react: { icon: GoTerminal, className: styles.defaultIcon }, svg: mdiConsole }],
    ['BASIC', { react: { icon: SiVisualbasic, className: styles.defaultIcon } }],
    ['C', { react: { icon: SiC, className: styles.blue }, svg: mdiLanguageC }],
    ['C++', { react: { icon: SiCplusplus, className: styles.blue }, svg: mdiLanguageCpp }],
    ['C#', { react: { icon: SiCsharp, className: styles.blue }, svg: mdiLanguageCsharp }],
    ['Clojure', { react: { icon: SiClojure, className: styles.blue } }],
    ['CMake', { react: { icon: SiCmake, className: styles.defaultIcon } }],
    ['CoffeeScript', { react: { icon: SiCoffeescript, className: styles.defaultIcon } }],

    // TODO: Decide icon for CSV?
    ['Crystal', { react: { icon: SiCrystal, className: styles.blue } }],
    ['CSS', { react: { icon: FaCss3Alt, className: styles.blue }, svg: mdiLanguageCss3 }],
    ['D', { react: { icon: SiD, className: styles.red } }],
    ['Dart', { react: { icon: SiDart, className: styles.blue } }],
    ['Dockerfile', { react: { icon: SiDocker, className: styles.blue }, svg: mdiDocker }],
    ['EditorConfig', { react: { icon: SiEditorconfig, className: styles.defaultIcon } }],
    ['Elixir', { react: { icon: SiElixir, className: styles.blue } }],
    ['Elm', { react: { icon: SiElm, className: styles.blue } }],
    ['Emacs Lisp', { react: { icon: SiGnuemacs, className: styles.defaultIcon } }],
    ['Erlang', { react: { icon: SiErlang, className: styles.blue } }],
    ['Fortran', { react: { icon: SiFortran, className: styles.defaultIcon } }],
    ['Fortran Free Form', { react: { icon: SiFortran, className: styles.defaultIcon } }],
    ['F#', { react: { icon: SiFsharp, className: styles.blue } }],
    ['Git Attributes', { react: { icon: SiGit, className: styles.red }, svg: mdiGit }],
    ['Go', { react: { icon: SiGo, className: styles.blue }, svg: mdiLanguageGo }],
    ['Go Module', { react: { icon: SiGo, className: styles.pink }, svg: mdiLanguageGo }],
    ['Go Checksums', { react: { icon: SiGo, className: styles.pink }, svg: mdiLanguageGo }],
    ['Groovy', { react: { icon: SiApachegroovy, className: styles.blue } }],
    ['GraphQL', { react: { icon: SiGraphql, className: styles.pink }, svg: mdiGraphql }],
    ['Haskell', { react: { icon: SiHaskell, className: styles.blue }, svg: mdiLanguageHaskell }],
    ['HTML', { react: { icon: SiHtml5, className: styles.blue }, svg: mdiLanguageHtml5 }],
    ['HTML+ECR', { react: { icon: SiCrystal, className: styles.blue } }],
    ['HTML+EEX', { react: { icon: SiElixir, className: styles.blue } }],
    ['HTML+ERB', { react: { icon: SiRuby, className: styles.blue } }],
    ['HTML+PHP', { react: { icon: SiPhp, className: styles.blue } }],
    ['HTML+Razor', { react: { icon: SiCsharp, className: styles.blue } }],
    ['Ignore List', { react: { icon: CiSettings, className: styles.defaultIcon }, svg: mdiCog }],
    ['Java', { react: { icon: GrJava, className: styles.defaultIcon }, svg: mdiLanguageJava }],
    ['JavaScript', { react: { icon: SiJavascript, className: styles.yellow }, svg: mdiLanguageJavascript }],
    ['Jinja', { react: { icon: SiJinja, className: styles.defaultIcon } }],
    ['JSON with Comments', { react: { icon: VscJson, className: styles.defaultIcon } }],
    ['JSON', { react: { icon: VscJson, className: styles.defaultIcon }, svg: mdiCodeJson }],
    ['JSON5', { react: { icon: VscJson, className: styles.defaultIcon }, svg: mdiCodeJson }],
    ['JSONLD', { react: { icon: VscJson, className: styles.defaultIcon }, svg: mdiCodeJson }],
    ['Julia', { react: { icon: SiJulia, className: styles.defaultIcon } }],
    ['Kotlin', { react: { icon: SiKotlin, className: styles.green }, svg: mdiLanguageKotlin }],
    ['LLVM', { react: { icon: SiLlvm, className: styles.gray } }],
    ['Lua', { react: { icon: SiLua, className: styles.blue }, svg: mdiLanguageLua }],
    ['Markdown', { react: { icon: SiMarkdown, className: styles.blue }, svg: mdiLanguageMarkdown }],
    ['Mathematica', { react: { icon: SiWolframmathematica, className: styles.red } }],

    // https://github.com/NCAR/ncl, not tweag/nickel
    ['NCL', { react: { icon: ImEarth, className: styles.defaultIcon }, svg: mdiEarth }],
    ['Nginx', { react: { icon: SiNginx, className: styles.defaultIcon } }],
    ['Nim', { react: { icon: SiNim, className: styles.yellow } }],
    ['Nix', { react: { icon: SiNixos, className: styles.gray }, svg: mdiNix }],
    ['NPM Config', { react: { icon: SiNpm, className: styles.red }, svg: mdiNpm }],

    // Missing an icon for Objective-C
    ['OCaml', { react: { icon: SiOcaml, className: styles.yellow } }],
    ['PHP', { react: { icon: SiPhp, className: styles.cyan }, svg: mdiLanguagePhp }],
    ['Perl', { react: { icon: SiPerl, className: styles.defaultIcon } }],
    ['PLpgSQL', { react: { icon: GoDatabase, className: styles.blue } }],
    ['PowerShell', { react: { icon: GoTerminal, className: styles.defaultIcon }, svg: mdiConsole }],

    // Missing icon for Protobuf
    ['PureScript', { react: { icon: SiPurescript, className: styles.defaultIcon } }],
    ['Python', { react: { icon: SiPython, className: styles.blue }, svg: mdiLanguagePython }],
    ['R', { react: { icon: SiR, className: styles.red }, svg: mdiLanguageR }],
    ['Ruby', { react: { icon: SiRuby, className: styles.red }, svg: mdiLanguageRuby }],
    ['Rust', { react: { icon: SiRust, className: styles.defaultIcon }, svg: mdiLanguageRust }],
    ['Scala', { react: { icon: SiScala, className: styles.red } }],
    ['Sass', { react: { icon: FaSass, className: styles.pink }, svg: mdiSass }],
    ['SCSS', { react: { icon: FaSass, className: styles.pink } }],
    ['SQL', { react: { icon: GoDatabase, className: styles.blue } }],

    // Missing icon for Starlark
    ['Svelte', { react: { icon: SiSvelte, className: styles.red } }],
    ['SVG', { react: { icon: SiSvg, className: styles.yellow }, svg: mdiSvg }],
    ['Swift', { react: { icon: SiSwift, className: styles.blue }, svg: mdiLanguageSwift }],
    ['Terraform', { react: { icon: SiTerraform, className: styles.blue } }],
    ['TSX', { react: { icon: SiTypescript, className: styles.blue }, svg: mdiLanguageTypescript }],
    ['TypeScript', { react: { icon: SiTypescript, className: styles.blue }, svg: mdiLanguageTypescript }],
    ['Text', { react: { icon: CiTextAlignLeft, className: styles.defaultIcon }, svg: mdiText }],

    // Missing icon for Thrift
    ['TOML', { react: { icon: SiToml, className: styles.defaultIcon } }],
    ['UnrealScript', { react: { icon: SiUnrealengine, className: styles.defaultIcon } }],
    ['VBA', { react: { icon: SiVisualbasic, className: styles.blue } }],
    ['VBScript', { react: { icon: SiVisualbasic, className: styles.blue } }],
    ['Vim Script', { react: { icon: SiVim, className: styles.defaultIcon } }],
    ['Vue', { react: { icon: FaVuejs, className: styles.green } }],
    ['WebAssembly', { react: { icon: SiWebassembly, className: styles.blue } }],
    ['XML', { react: { icon: CiSettings, className: styles.defaultIcon }, svg: mdiCog }],
    ['YAML', { react: { icon: CiSettings, className: styles.defaultIcon }, svg: mdiCog }],
    ['Zig', { react: { icon: SiZig, className: styles.yellow } }],
])

export interface BinaryFileIcon {
    react: ReactIcon
    svg: string
}

/**
 * DO NOT add any extensions here for which there are multiple different
 * file formats in practice which use the same extensions.
 *
 * For programming languages, update {@link FILE_ICONS_BY_LANGUAGE}.
 */
const BINARY_FILE_ICONS_BY_EXTENSION: Map<string, BinaryFileIcon> = new Map([
    ['gif', { react: { icon: MdGif, className: styles.defaultIcon }, svg: mdiFileGifBox }],
    ['giff', { react: { icon: MdGif, className: styles.defaultIcon }, svg: mdiFileGifBox }],
    ['jpg', { react: { icon: SiJpeg, className: styles.yellow }, svg: mdiFileJpgBox }],
    ['jpeg', { react: { icon: SiJpeg, className: styles.yellow }, svg: mdiFileJpgBox }],
    ['png', { react: { icon: PiFilePngLight, className: styles.defaultIcon }, svg: mdiFilePngBox }],
])

// See TODO(id: md-icons-and-react-icons) for context
export interface IconInfo {
    // For use in the React webapp
    react: ReactIcon

    // For use in the SvelteKit rewrite
    svg: SvgIcon
}

/**
 *
 * See FIXME(id: language-detection) for context on why this takes a
 * languages argument instead of directly using the file extension
 * for determining the language.
 *
 * @param path The path of the file (or just its name).
 * @param language Alias to the file language name.
 * @returns undefined if the language is not a known language.
 */
export function getFileIconInfo(path: string, language: string): IconInfo | undefined {
    const extension = path.split('.').at(-1) ?? ''
    const icon1 = BINARY_FILE_ICONS_BY_EXTENSION.get(extension)

    if (icon1 !== undefined) {
        return {
            react: icon1.react,
            svg: { path: icon1.svg, color: classNameToColor(icon1.react.className) },
        }
    }

    const icon2 = FILE_ICONS_BY_LANGUAGE.get(language)

    if (icon2 !== undefined) {
        return {
            react: icon2.react,
            svg: { path: icon2.svg ?? mdiFileCodeOutline, color: classNameToColor(icon2.react.className) },
        }
    }

    return
}

const BLUE = 'var(--blue)'
const PINK = 'var(--pink)'
const YELLOW = 'var(--yellow)'
const RED = 'var(--red)'
const GREEN = 'var(--green)'
const CYAN = 'var(--blue)'
const GRAY = 'var(--gray-05)'

function classNameToColor(name: string): string {
    switch (name) {
        case styles.blue: {
            return BLUE
        }
        case styles.red: {
            return RED
        }
        case styles.yellow: {
            return YELLOW
        }
        case styles.pink: {
            return PINK
        }
        case styles.green: {
            return GREEN
        }
        case styles.cyan: {
            return CYAN
        }
        case styles.gray:
        default: {
            return GRAY
        }
    }
}
