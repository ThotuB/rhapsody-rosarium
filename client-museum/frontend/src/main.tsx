import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { Interface } from './Interface'
import { preventPageZoom } from './noZoom'
import { Projector } from './Projector'

preventPageZoom()

// both windows load this page; the projector's url has ?view=projector
const projector = new URLSearchParams(location.search).get('view') === 'projector'

createRoot(document.getElementById('root')!).render(
    <StrictMode>{projector ? <Projector /> : <Interface />}</StrictMode>,
)
