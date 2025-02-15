/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { Box, ButtonPrimary, Flex, Text } from 'design';
import { MenuIcon, MenuItem } from 'shared/components/MenuAction';
import { GitHubIcon } from 'design/SVGIcon';

import { State as ResourceState } from 'teleport/components/useResources';

import { ResponsiveConnector } from 'teleport/AuthConnectors/styles/ConnectorBox.styles';

import { State as AuthConnectorState } from '../useAuthConnectors';

export default function ConnectorList({ items, onEdit, onDelete }: Props) {
  items = items || [];
  const $items = items.map(item => {
    const { id, name } = item;
    return (
      <ConnectorListItem
        key={id}
        id={id}
        onEdit={onEdit}
        onDelete={onDelete}
        name={name}
      />
    );
  });

  return (
    <Flex flexWrap="wrap" alignItems="center" flex={1} gap={5}>
      {$items}
    </Flex>
  );
}

function ConnectorListItem({ name, id, onEdit, onDelete }) {
  const onClickEdit = () => onEdit(id);
  const onClickDelete = () => onDelete(id);

  return (
    <ResponsiveConnector>
      <Flex width="100%" justifyContent="center">
        <MenuIcon buttonIconProps={menuActionProps}>
          <MenuItem onClick={onClickDelete}>Delete...</MenuItem>
        </MenuIcon>
      </Flex>
      <Flex
        flex="1"
        alignItems="center"
        justifyContent="center"
        flexDirection="column"
        width="200px"
        style={{ textAlign: 'center' }}
      >
        <Box mb={3} mt={3}>
          <GitHubIcon style={{ textAlign: 'center' }} size={50} />
        </Box>
        <Text style={{ width: '100%' }} typography="body2" bold caps>
          {name}
        </Text>
      </Flex>
      <ButtonPrimary mt="auto" size="medium" block onClick={onClickEdit}>
        EDIT CONNECTOR
      </ButtonPrimary>
    </ResponsiveConnector>
  );
}

const menuActionProps = {
  style: {
    right: '10px',
    position: 'absolute',
    top: '10px',
  },
};

type Props = {
  items: AuthConnectorState['items'];
  onEdit: ResourceState['edit'];
  onDelete: ResourceState['remove'];
};
