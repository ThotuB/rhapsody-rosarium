import { Icon, type IconName } from './components/icons'
import { controlIntro, IntroAction, useIntro } from './useMuseum'

// The projector's opening video, controlled from here: the projection itself has no controls.
export function IntroControls() {
    const intro = useIntro()
    const onMap = intro?.phase === 'map'
    const playing = !onMap && !!intro?.playing
    const muted = !!intro?.muted

    return (
        <div className="mt-3 flex items-center gap-2" role="group" aria-label="Opening video on the projector">
            <IconButton
                icon={onMap || playing ? 'replay' : 'play'}
                label={onMap ? 'Replay intro' : playing ? 'Restart intro' : 'Start intro'}
                onClick={() => controlIntro(IntroAction.IntroStart)}
            />
            <IconButton
                icon={muted ? 'muted' : 'sound'}
                label={muted ? 'Turn intro sound on' : 'Mute intro'}
                onClick={() => controlIntro(muted ? IntroAction.IntroUnmute : IntroAction.IntroMute)}
                disabled={onMap}
            />
            <IconButton
                icon="skip"
                label="Skip to the map"
                onClick={() => controlIntro(IntroAction.IntroSkip)}
                disabled={onMap}
            />
            <span className="text-sm text-muted">
                {onMap ? 'Projector on the map' : playing ? 'Intro playing' : 'Intro ready'}
            </span>
        </div>
    )
}

// an icon-only control; the label is what screen readers say, and the tooltip
function IconButton({
    icon,
    label,
    onClick,
    disabled = false,
}: {
    icon: IconName
    label: string
    onClick: () => void
    disabled?: boolean
}) {
    return (
        <button
            type="button"
            className="grid size-10 cursor-pointer place-items-center rounded-full border border-line hover:bg-rose/10 disabled:cursor-default disabled:opacity-35 disabled:hover:bg-transparent [&_svg]:size-5"
            onClick={onClick}
            disabled={disabled}
            aria-label={label}
            title={label}
        >
            <Icon name={icon} />
        </button>
    )
}
