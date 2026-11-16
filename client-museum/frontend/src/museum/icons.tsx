// Small line icons for the intro controls, drawn in the text colour.

const paths = {
    play: <path d="M7 4.5v15l12-7.5z" fill="currentColor" />,
    sound: (
        <>
            <path d="M4 9.5h3.5L12 5.5v13l-4.5-4H4z" fill="currentColor" />
            <path d="M15.5 9a4 4 0 0 1 0 6M18 6.5a7.5 7.5 0 0 1 0 11" />
        </>
    ),
    muted: (
        <>
            <path d="M4 9.5h3.5L12 5.5v13l-4.5-4H4z" fill="currentColor" />
            <path d="M15.5 9.5l5 5M20.5 9.5l-5 5" />
        </>
    ),
    skip: (
        <>
            <path d="M5 5v14l10-7z" fill="currentColor" />
            <path d="M18.5 5v14" />
        </>
    ),
    replay: (
        <>
            <path d="M4.5 12a7.5 7.5 0 1 0 2.2-5.3" />
            <path d="M4.5 3.5v4.5H9" />
        </>
    ),
}

export type IconName = keyof typeof paths

export function Icon({ name }: { name: IconName }) {
    return (
        <svg
            viewBox="0 0 24 24"
            aria-hidden="true"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
        >
            {paths[name]}
        </svg>
    )
}
