// The entry point of both windows: they load the same page, and the projector's url has ?view=projector.
//   museum/     the museum window, on the laptop: the stations, the map, the intro's controls
//   projector/  the projector window: the opening video, then the map and the live piano keyboard
//   shared/     used by both: the map, and what talks to the Go side (useMuseum)
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { MuseumWindow } from './museum/MuseumWindow'
import { preventPageZoom } from './noZoom'
import { ProjectorWindow } from './projector/ProjectorWindow'

preventPageZoom()

const projector = new URLSearchParams(location.search).get('view') === 'projector'

createRoot(document.getElementById('root')!).render(
    <StrictMode>{projector ? <ProjectorWindow /> : <MuseumWindow />}</StrictMode>,
)
