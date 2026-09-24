-- Filtro de pandoc para el PDF de los informes de arquitectura.
--
-- Los diagramas llegan como una imagen sola dentro de un párrafo, que es lo que
-- escribe mermaid-cli al sustituir el bloque de código. Se centran.

function Para(el)
  if #el.content ~= 1 or el.content[1].t ~= 'Image' then return nil end
  return {
    pandoc.RawBlock('latex', '\\begin{center}'),
    el,
    pandoc.RawBlock('latex', '\\end{center}'),
  }
end
