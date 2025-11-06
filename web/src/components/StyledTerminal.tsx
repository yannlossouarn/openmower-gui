import styled from "styled-components";

export const StyledTerminal = styled.div`
  div.react-terminal-wrapper {
    padding-top: 35px;
    padding-bottom: 8px;
    padding-left: 8px;
    padding-right: 8px;
    background-color: #1a1e24 !important;
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