#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import io
import logging
import os
import re
import stat
import sys
import tarfile
from base64 import b64decode
from collections import defaultdict
from os.path import basename, normpath
from shutil import copyfileobj

import boto3
import click
import requests
from yaml import dump, load

try:
    from yaml import CDumper as Dumper
    from yaml import CLoader as Loader
except ImportError:
    from yaml import Dumper, Loader


IMAGES = ['kube-apiserver', 'kube-controller-manager', 'kube-scheduler', 'pause', 'etcd']
COMPONENTS = ['kube-proxy', 'coredns', 'metrics-server']
ECR_REGEX = r'(?P<registry>.+)\.dkr\.ecr\.(?P<region>.+)\.amazonaws\.com'


@click.command(context_settings={'max_content_width': 120})
@click.option('--release-url', type=str, required=True, help='URL to a Kubernetes Release YAML')
@click.option('--data-dir', type=str, default='/var/lib/rancher/rke2', help='RKE2 data directory', show_default=True)
@click.option('--prefix', type=str, default='/', help='Prefix for output files created by this script', show_default=True)
def main(release_url, data_dir, prefix):
    release = get_release(release_url)

    logging.info(f'Got Release: {release["metadata"]["name"]}')

    rke2_config = {
            'kubelet-path': normpath(f'{data_dir}/opt/bin/kubelet'),
            'data-dir': normpath(data_dir),
            }

    try:
        write_ecr_credentials(release, prefix)
    except Exception as e:
        raise Exception(f'Unable to write ECR credentials to registries.yaml: {e}') from e

    for image in IMAGES:
        alt_image = get_image_artifact(release, image)
        rke2_config[f'{image}-image'] = alt_image

    for component in COMPONENTS:
        alt_image = get_image_artifact(release, component)
        write_chart_config(component, alt_image, f'{prefix}/{data_dir}')

    extract_archive(release, 'kubernetes-node-linux-amd64.tar.gz', f'{prefix}/{data_dir}')

    write_rke2_config(rke2_config, prefix)


def get_release(release_url):
    response = requests.get(release_url)
    response.raise_for_status()
    return load(response.text, Loader=Loader)


def get_image_artifact(release, component):
    for c in release.get('status', {}).get('components', []):
        for a in c.get('assets', []):
            if a['name'] == component + '-image':
                return a['image']['uri']

    raise Exception(f'Unable to find image asset for {component}')


def write_rke2_config(config, prefix):
    etc_dir = f'{prefix}/etc/rancher/rke2'

    try:
        os.makedirs(normpath(etc_dir), mode=0o0700)
    except FileExistsError:
        pass

    with open(normpath(f'{etc_dir}/config.yaml'), mode='a+') as config_file:
        config_file.seek(0, 0)
        config_yaml = load(config_file, Loader=Loader)
        if not config_yaml:
            config_yaml = dict()
        config_yaml.update(config)
        logging.info(f'Writing config to {config_file.name}')
        config_file.seek(0, 0)
        config_file.truncate()
        dump(config_yaml, config_file, Dumper=Dumper)


def write_chart_config(component, image, data_dir):
    manifests_dir = f'{data_dir}/server/manifests'
    repository, tag = image.split(':')

    values_yaml = {
        'image': {
            'repository': repository,
            'tag': tag,
            },
        }
    config_yaml = {
        'apiVersion': 'helm.cattle.io/v1',
        'kind': 'HelmChartConfig',
        'metadata': {
            'name': f'rke2-{component}',
            'namespace': 'kube-system',
            },
        'spec': {
            'valuesContent': dump(values_yaml, Dumper=Dumper),
            },
        }

    try:
        os.makedirs(normpath(manifests_dir), mode=0o0700)
    except FileExistsError:
        pass

    with open(normpath(f'{manifests_dir}/rke2-{component}-config.yaml'), mode='w+') as config_file:
        logging.info(f'Writing HelmChartConfig to {config_file.name}')
        dump(config_yaml, config_file, Dumper=Dumper)


def extract_archive(release, name, data_dir):
    bin_dir = normpath(f'{data_dir}/opt/bin')

    try:
        os.makedirs(normpath(bin_dir), mode=0o0700)
    except FileExistsError:
        pass

    for c in release.get('status', {}).get('components', []):
        for a in c.get('assets', []):
            if a['type'] == 'Archive' and a['name'] == name:
                extract_network_tar(a['archive']['uri'], bin_dir)


def extract_network_tar(archive_url, bin_dir):
    logging.info(f'Extracting files from {archive_url}')
    with requests.get(archive_url, stream=True) as response, io.BytesIO() as buf:
        response.raise_for_status()
        copyfileobj(response.raw, buf)
        buf.seek(0, 0)

        with tarfile.open(fileobj=buf) as tar:
            for member in tar.getmembers():
                if member.isreg() and member.mode & stat.S_IXUSR:
                    member.name = basename(member.name)
                    logging.info(f'Extracting {bin_dir}/{member.name}')
                    tar.extract(member, bin_dir)


def write_ecr_credentials(release, prefix):
    etc_dir = f'{prefix}/etc/rancher/rke2'
    registries = get_ecr_registries(release)

    registry_configs = dict()
    registry_regions = defaultdict(list)

    for registry in registries:
        match = re.match(ECR_REGEX, registry)
        if match:
            registry_regions[match.group('region')].append(match.group('registry'))

    if not registry_regions:
        return

    for region, registries in registry_regions.items():
        logging.info(f'Getting auth tokens for {registries} in {region}')
        response = boto3.client('ecr', region_name=region).get_authorization_token(registryIds=registries)
        for auth in response.get('authorizationData', []):
            endpoint = auth['proxyEndpoint'].split('//')[1]
            username, password = b64decode(auth['authorizationToken']).decode().split(':')
            registry_configs[endpoint] = {'auth': {'username': username, 'password': password}}

    try:
        os.makedirs(normpath(etc_dir), mode=0o0700)
    except FileExistsError:
        pass

    with open(normpath(f'{etc_dir}/registries.yaml'), mode='a+') as registries_file:
        registries_file.seek(0, 0)
        registries_yaml = load(registries_file, Loader=Loader)
        if not registries_yaml:
            registries_yaml = dict()
        if 'configs' not in registries_yaml:
            registries_yaml['configs'] = dict()
        registries_yaml['configs'].update(registry_configs)
        logging.info(f'Writing credentials to {registries_file.name}')
        registries_file.seek(0, 0)
        registries_file.truncate()
        dump(registries_yaml, registries_file, Dumper=Dumper)


def get_ecr_registries(release):
    registries = set()
    for c in release.get('status', {}).get('components', []):
        for a in c.get('assets', []):
            if 'image' in a:
                registry = a['image']['uri'].split('/')[0]
                if '.ecr.' in registry:
                    registries.add(registry)
    return registries


if __name__ == '__main__':
    try:
        logging.basicConfig(format="%(levelname).1s %(message)s", level=logging.INFO, stream=sys.stdout)
        main()
    except (KeyboardInterrupt, BrokenPipeError):
        pass
    except Exception as e:
        logging.fatal(e)
