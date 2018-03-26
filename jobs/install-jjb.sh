#!/bin/bash

yum install wget -y
wget https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm
yum install epel-release-latest-7.noarch.rpm -y
yum install python-pip -y
pip install --upgrade pip
pip install jenkins-job-builder
echo 'PATH=$HOME/.local/bin:$PATH' > ~/.bashrc
