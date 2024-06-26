export type Key =
    | AlphabetKey
    | NumericKey
    | SymbolKey
    | WhiteSpaceKey
    | NavigationKey
    | EditingKey
    | UIKey
    | DeviceKey
    | IMEKey
    | FunctionKey
    | PhoneKey
    | MultimediaKey
    | AudioControlKey
    | TVControlKey
    | MediaControllerKey
    | SpeechRecognitionKey
    | DocumentKey
    | ApplicationSelectorKey
    | BrowserControlKey

export type AlphabetKey =
    | 'a'
    | 'b'
    | 'c'
    | 'd'
    | 'e'
    | 'f'
    | 'g'
    | 'h'
    | 'i'
    | 'j'
    | 'k'
    | 'l'
    | 'm'
    | 'n'
    | 'o'
    | 'p'
    | 'q'
    | 'r'
    | 's'
    | 't'
    | 'u'
    | 'v'
    | 'w'
    | 'x'
    | 'y'
    | 'z'
    | 'A'
    | 'B'
    | 'C'
    | 'D'
    | 'E'
    | 'F'
    | 'G'
    | 'H'
    | 'I'
    | 'J'
    | 'K'
    | 'L'
    | 'M'
    | 'N'
    | 'O'
    | 'P'
    | 'Q'
    | 'R'
    | 'S'
    | 'T'
    | 'U'
    | 'V'
    | 'W'
    | 'X'
    | 'Y'
    | 'Z'

export type NumericKey = '0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9'

export const MODIFIER_KEYS = [
    'Alt',
    'AltGraph',
    'CapsLock',
    'Control',
    'Fn',
    'FnLock',
    'Hyper',
    'Meta',
    'Shift',
    'Super',
    'Symbol',
    // We are explicitly excluding these keys because they usually should not impact
    // they keys used for keyboard shortcuts. We've gotten reports that single letter
    // keyboard shortcuts are not working as expected when e.g. num lock is on.
    // 'SymbolLock',
    // 'NumLock',
    // 'ScrollLock',
] as const

export type ModifierKey = typeof MODIFIER_KEYS[number]

export type SymbolKey =
    | '~'
    | '`'
    | '!'
    | '@'
    | '#'
    | '$'
    | '%'
    | '^'
    | '&'
    | '*'
    | '('
    | ')'
    | '-'
    | '_'
    | '+'
    | '='
    | '['
    | ']'
    | '{'
    | '}'
    | '\\'
    | '|'
    | ';'
    | ':'
    | "'"
    | '"'
    | ','
    | '<'
    | '.'
    | '>'
    | '/'
    | '?'

export type WhiteSpaceKey = 'Enter' | 'Tab' | ' '

export type NavigationKey =
    | 'ArrowDown'
    | 'ArrowLeft'
    | 'ArrowRight'
    | 'ArrowUp'
    | 'End'
    | 'Home'
    | 'PageUp'
    | 'PageDown'

export type EditingKey =
    | 'Backspace'
    | 'Clear'
    | 'Copy'
    | 'CrSel'
    | 'Cut'
    | 'Delete'
    | 'EraseEof'
    | 'ExSel'
    | 'Insert'
    | 'Paste'
    | 'Redo'
    | 'Undo'

export type UIKey =
    | 'Accept'
    | 'Again'
    | 'Attn'
    | 'Cancel'
    | 'ContextMenu'
    | 'Escape'
    | 'Execute'
    | 'Find'
    | 'Finish'
    | 'Help'
    | 'Pause'
    | 'Play'
    | 'Props'
    | 'Select'
    | 'ZoomIn'
    | 'ZoomOut'

export type DeviceKey =
    | 'BrightnessDown'
    | 'BrightnessUp'
    | 'Eject'
    | 'LogOff'
    | 'Power'
    | 'PowerOff'
    | 'PrintScreen'
    | 'Hibernate'
    | 'Standby'
    | 'WakeUp'

export type IMEKey =
    | 'AllCandidates'
    | 'Alphanumeric'
    | 'CodeInput'
    | 'Compose'
    | 'Convert'
    | 'Dead'
    | 'FinalMode'
    | 'GroupFirst'
    | 'GroupLast'
    | 'GroupNext'
    | 'GroupPrevious'
    | 'ModeChange'
    | 'NextCandidate'
    | 'NonConvert'
    | 'PreviousCandidate'
    | 'Process'
    | 'SingleCandidate'

export type FunctionKey =
    | 'F1'
    | 'F2'
    | 'F3'
    | 'F4'
    | 'F5'
    | 'F6'
    | 'F7'
    | 'F8'
    | 'F9'
    | 'F10'
    | 'F11'
    | 'F12'
    | 'F13'
    | 'F14'
    | 'F15'
    | 'F16'
    | 'F17'
    | 'F18'
    | 'F19'
    | 'F20'
    | 'Soft1'
    | 'Soft2'
    | 'Soft3'
    | 'Soft4'

export type PhoneKey =
    | 'AppSwitch'
    | 'Call'
    | 'Camera'
    | 'CameraFocus'
    | 'EndCall'
    | 'GoBack'
    | 'GoHome'
    | 'HeadsetHook'
    | 'LastNumberRedial'
    | 'Notification'
    | 'MannerMode'
    | 'VoiceDial'

export type MultimediaKey =
    | 'ChannelUp'
    | 'ChannelDown'
    | 'MediaFastForward'
    | 'MediaPause'
    | 'MediaPlay'
    | 'MediaPlayPause'
    | 'MediaRecord'
    | 'MediaRewind'
    | 'MediaStop'
    | 'MediaTrackNext'
    | 'MediaTrackPrevious'

export type AudioControlKey =
    | 'AudioBalanceLeft'
    | 'AudioBalanceRight'
    | 'AudioBassDown'
    | 'AudioBassBoostDown'
    | 'AudioBassBoostToggle'
    | 'AudioBassBoostUp'
    | 'AudioBassUp'
    | 'AudioFaderFront'
    | 'AudioFaderRear'
    | 'AudioSurroundModeNext'
    | 'AudioTrebleDown'
    | 'AudioTrebleUp'
    | 'AudioVolumeDown'
    | 'AudioVolumeMute'
    | 'AudioVolumeUp'
    | 'MicrophoneVolumeDown'
    | 'MicrophoneVolumeMute'
    | 'MicrophoneVolumeUp'

export type TVControlKey =
    | 'TV'
    | 'TV3DMode'
    | 'TVAntennaCable'
    | 'TVAudioDescription'
    | 'TVAudioDescriptionMixDown'
    | 'TVAudioDescriptionMixUp'
    | 'TVContentsMenu'
    | 'TVDataService'
    | 'TVInput'
    | 'TVInputComponent1'
    | 'TVInputComponent2'
    | 'TVInputComposite1'
    | 'TVInputComposite2'
    | 'TVInputHDMI1'
    | 'TVInputHDMI2'
    | 'TVInputHDMI3'
    | 'TVInputHDMI4'
    | 'TVInputVGA1'
    | 'TVMediaContext'
    | 'TVNetwork'
    | 'TVNumberEntry'
    | 'TVPower'
    | 'TVRadioService'
    | 'TVSatellite'
    | 'TVSatelliteBS'
    | 'TVSatelliteCS'
    | 'TVSatelliteToggle'
    | 'TVTerrestrialAnalog'
    | 'TVTerrestrialDigital'
    | 'TVTimer'

export type MediaControllerKey =
    | 'AVRInput'
    | 'AVRPower'
    | 'ColorF0Red'
    | 'ColorF1Green'
    | 'ColorF2Yellow'
    | 'ColorF3Blue'
    | 'ColorF4Grey'
    | 'ColorF5Brown'
    | 'ClosedCaptionToggle'
    | 'Dimmer'
    | 'DisplaySwap'
    | 'DVR'
    | 'Exit'
    | 'FavoriteClear0'
    | 'FavoriteClear1'
    | 'FavoriteClear2'
    | 'FavoriteClear3'
    | 'FavoriteRecall0'
    | 'FavoriteRecall1'
    | 'FavoriteRecall2'
    | 'FavoriteRecall3'
    | 'FavoriteStore0'
    | 'FavoriteStore1'
    | 'FavoriteStore2'
    | 'FavoriteStore3'
    | 'Guide'
    | 'GuideNextDay'
    | 'GuidePreviousDay'
    | 'Info'
    | 'InstantReplay'
    | 'Link'
    | 'ListProgram'
    | 'LiveContent'
    | 'Lock'
    | 'MediaApps'
    | 'MediaAudioTrack'
    | 'MediaLast'
    | 'MediaSkipBackward'
    | 'MediaSkipForward'
    | 'MediaStepBackward'
    | 'MediaStepForward'
    | 'MediaTopMenu'
    | 'NavigateIn'
    | 'NavigateNext'
    | 'NavigateOut'
    | 'NavigatePrevious'
    | 'NextFavoriteChannel'
    | 'NextUserProfile'
    | 'OnDemand'
    | 'Pairing'
    | 'PinPDown'
    | 'PinPMove'
    | 'PinPToggle'
    | 'PinPUp'
    | 'PlaySpeedDown'
    | 'PlaySpeedReset'
    | 'PlaySpeedUp'
    | 'RandomToggle'
    | 'RcLowBattery'
    | 'RecordSpeedNext'
    | 'RfBypass'
    | 'ScanChannelsToggle'
    | 'ScreenModeNext'
    | 'Settings'
    | 'SplitScreenToggle'
    | 'STBInput'
    | 'STBPower'
    | 'Subtitle'
    | 'Teletext'
    | 'VideoModeNext'
    | 'Wink'
    | 'ZoomToggle'

export type SpeechRecognitionKey = 'SpeechCorrectionList' | 'SpeechInputToggle'

export type DocumentKey =
    | 'Close'
    | 'New'
    | 'Open'
    | 'Print'
    | 'Save'
    | 'SpellCheck'
    | 'MailForward'
    | 'MailReply'
    | 'MailSend'

export type ApplicationSelectorKey =
    | 'LaunchCalculator'
    | 'LaunchCalendar'
    | 'LaunchContacts'
    | 'LaunchMail'
    | 'LaunchMediaPlayer'
    | 'LaunchMusicPlayer'
    | 'LaunchMyComputer'
    | 'LaunchPhone'
    | 'LaunchScreenSaver'
    | 'LaunchSpreadsheet'
    | 'LaunchWebBrowser'
    | 'LaunchWebCam'
    | 'LaunchWordProcessor'
    | 'LaunchApplication1'
    | 'LaunchApplication2'
    | 'LaunchApplication3'
    | 'LaunchApplication4'
    | 'LaunchApplication5'
    | 'LaunchApplication6'
    | 'LaunchApplication7'
    | 'LaunchApplication8'
    | 'LaunchApplication9'
    | 'LaunchApplication10'
    | 'LaunchApplication11'
    | 'LaunchApplication12'
    | 'LaunchApplication13'
    | 'LaunchApplication14'
    | 'LaunchApplication15'
    | 'LaunchApplication16'

export type BrowserControlKey =
    | 'BrowserBack'
    | 'BrowserFavorites'
    | 'BrowserForward'
    | 'BrowserHome'
    | 'BrowserRefresh'
    | 'BrowserSearch'
    | 'BrowserStop'
