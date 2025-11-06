import styled from "styled-components";

export const StyledTerminal = styled.div`
  div.react-terminal-wrapper {
    padding-top: 35px;
    background-color: #000 !important;
    font-size: 13px;
  }

  div.react-terminal-wrapper > div.react-terminal-window-buttons {
    display: none;
  }

  div.react-terminal {
    height: auto !important;
  }

  div.react-terminal-light .react-terminal-line::before {
    display: initial !important;
    content: initial !important;
  }
`;